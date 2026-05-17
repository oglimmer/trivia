// Package metrics exposes a Prometheus /metrics endpoint plus the
// instrumentation helpers used across the API. Counters/histograms are
// pushed by callers at the point of interest; gauges that depend on live
// state (online players, games by state) are pulled on every scrape via
// caller-supplied closures so this package stays free of DB/Hub deps.
package metrics

import (
	"bufio"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/oglimmer/trivia/backend/internal/buildinfo"
)

// Options carries pull-style data sources the metrics package can't compute
// on its own. Both closures, if provided, are invoked on every scrape.
type Options struct {
	// OnlinePlayers reports the total distinct online players across all
	// games. May be nil to omit the gauge.
	OnlinePlayers func() int
	// GamesByState reports a state -> count map (e.g. setup, game, finished).
	// May be nil to omit the gauge.
	GamesByState func() map[string]int
}

type Metrics struct {
	Registry *prometheus.Registry

	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec
	HTTPInFlight prometheus.Gauge

	WSConnections     *prometheus.GaugeVec
	WSSessionDuration *prometheus.HistogramVec

	AnswersSubmitted   *prometheus.CounterVec
	QuestionsActivated prometheus.Counter
	QuestionsRevealed  prometheus.Counter
	QuestionsAutoClose prometheus.Counter

	AISuggestRequests *prometheus.CounterVec
	AISuggestDuration prometheus.Histogram

	ImagesUploaded prometheus.Counter
	OrphansDeleted prometheus.Counter
}

func New(opts Options) *Metrics {
	reg := prometheus.NewRegistry()

	// Go runtime + OS process collectors — standard best-practice baseline.
	reg.MustRegister(
		collectors.NewGoCollector(
			collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection|collectors.GoRuntimeMemStatsCollection),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{Registry: reg}

	m.HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "http",
		Name: "requests_total",
		Help: "Total HTTP requests served, labelled by method, route pattern and status.",
	}, []string{"method", "path", "status"})

	m.HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trivia", Subsystem: "http",
		Name:    "request_duration_seconds",
		Help:    "HTTP request duration in seconds, labelled by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	m.HTTPInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "trivia", Subsystem: "http",
		Name: "in_flight_requests",
		Help: "Number of HTTP requests currently being served.",
	})

	m.WSConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trivia", Subsystem: "ws",
		Name: "connections",
		Help: "Currently connected WebSocket clients by role (player|admin).",
	}, []string{"role"})
	m.WSSessionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trivia", Subsystem: "ws",
		Name:    "session_duration_seconds",
		Help:    "Lifetime of a WebSocket session in seconds, labelled by role.",
		Buckets: []float64{1, 10, 30, 60, 300, 900, 1800, 3600, 7200},
	}, []string{"role"})

	m.AnswersSubmitted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "game",
		Name: "answers_submitted_total",
		Help: "Player answers accepted, labelled by result (correct|wrong).",
	}, []string{"result"})

	m.QuestionsActivated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "game",
		Name: "questions_activated_total",
		Help: "Questions activated (admin manually or auto-next).",
	})
	m.QuestionsRevealed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "game",
		Name: "questions_revealed_total",
		Help: "Questions revealed via admin reveal action.",
	})
	m.QuestionsAutoClose = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "game",
		Name: "questions_auto_closed_total",
		Help: "Questions that ended because the auto-close timer fired.",
	})

	m.AISuggestRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "ai",
		Name: "suggest_requests_total",
		Help: "AI question-suggestion requests by outcome (success|error).",
	}, []string{"result"})
	m.AISuggestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "trivia", Subsystem: "ai",
		Name:    "suggest_duration_seconds",
		Help:    "AI question-suggestion latency in seconds.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 90, 120},
	})

	m.ImagesUploaded = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "images",
		Name: "uploaded_total",
		Help: "Successful image uploads.",
	})
	m.OrphansDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "trivia", Subsystem: "images",
		Name: "orphans_deleted_total",
		Help: "Orphan images removed by the background GC sweep.",
	})

	reg.MustRegister(
		m.HTTPRequests, m.HTTPDuration, m.HTTPInFlight,
		m.WSConnections, m.WSSessionDuration,
		m.AnswersSubmitted,
		m.QuestionsActivated, m.QuestionsRevealed, m.QuestionsAutoClose,
		m.AISuggestRequests, m.AISuggestDuration,
		m.ImagesUploaded, m.OrphansDeleted,
	)

	// build_info: value is always 1; labels carry the metadata so a single
	// PromQL `trivia_build_info` shows the deployed version.
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trivia",
		Name:      "build_info",
		Help:      "Build metadata. The metric value is always 1; labels carry the data.",
	}, []string{"version", "commit", "build_time", "go_version"})
	buildInfo.WithLabelValues(buildinfo.Version, buildinfo.Commit, buildinfo.Time, runtime.Version()).Set(1)
	reg.MustRegister(buildInfo)

	if opts.OnlinePlayers != nil {
		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "trivia", Subsystem: "game",
			Name: "online_players",
			Help: "Distinct online players across all games (sum across rooms).",
		}, func() float64 { return float64(opts.OnlinePlayers()) }))
	}
	if opts.GamesByState != nil {
		reg.MustRegister(&gamesByStateCollector{
			desc: prometheus.NewDesc(
				"trivia_game_count",
				"Number of games by state.",
				[]string{"state"}, nil,
			),
			fn: opts.GamesByState,
		})
	}

	return m
}

// gamesByStateCollector lets us emit a labelled gauge that's recomputed on
// every scrape — GaugeFunc only supports a single unlabelled series.
type gamesByStateCollector struct {
	desc *prometheus.Desc
	fn   func() map[string]int
}

func (c *gamesByStateCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *gamesByStateCollector) Collect(ch chan<- prometheus.Metric) {
	for state, count := range c.fn() {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(count), state)
	}
}

// Handler returns the /metrics HTTP handler. The token is matched against the
// request's Authorization: Bearer header in constant time. An empty token
// disables the endpoint entirely (404) — fail-closed beats accidentally
// shipping metrics open.
func (m *Metrics) Handler(token string) http.Handler {
	prom := promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{Registry: m.Registry})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.NotFound(w, r)
			return
		}
		got := bearerToken(r)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		prom.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// InstrumentHTTP records request count, in-flight, and latency. It uses the
// chi route pattern (e.g. /api/games/{code}) as the path label so game codes
// and image UUIDs don't blow up label cardinality. Must be mounted via
// `r.Use(...)` inside the chi router so the route pattern is available.
func (m *Metrics) InstrumentHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.HTTPInFlight.Inc()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		m.HTTPInFlight.Dec()

		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = "other"
		}
		status := strconv.Itoa(sr.status)
		m.HTTPRequests.WithLabelValues(r.Method, path, status).Inc()
		m.HTTPDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.wroteHeader {
		sr.status = code
		sr.wroteHeader = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter so the gorilla websocket
// upgrader (which type-asserts to http.Hijacker) keeps working through this
// wrapper. Without this the /ws upgrade fails with "response does not
// implement http.Hijacker".
func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := sr.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("response does not implement http.Hijacker")
}

// Flush forwards to the underlying ResponseWriter for streaming handlers.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// WS connection helpers.
func (m *Metrics) WSConnect(role string)    { m.WSConnections.WithLabelValues(role).Inc() }
func (m *Metrics) WSDisconnect(role string) { m.WSConnections.WithLabelValues(role).Dec() }

// RecordWSSession observes the lifetime of a WebSocket session.
func (m *Metrics) RecordWSSession(role string, d time.Duration) {
	m.WSSessionDuration.WithLabelValues(role).Observe(d.Seconds())
}

// RecordAnswer increments the answer counter by result.
func (m *Metrics) RecordAnswer(correct bool) {
	if correct {
		m.AnswersSubmitted.WithLabelValues("correct").Inc()
		return
	}
	m.AnswersSubmitted.WithLabelValues("wrong").Inc()
}

// RecordAISuggest records the outcome and duration of an AI suggestion call.
func (m *Metrics) RecordAISuggest(err error, d time.Duration) {
	result := "success"
	if err != nil {
		result = "error"
	}
	m.AISuggestRequests.WithLabelValues(result).Inc()
	m.AISuggestDuration.Observe(d.Seconds())
}
