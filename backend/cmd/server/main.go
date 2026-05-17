package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/api"
	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/images"
	"github.com/oglimmer/trivia/backend/internal/mail"
	"github.com/oglimmer/trivia/backend/internal/metrics"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

func main() {
	ctx := context.Background()

	d, err := connectWithRetry(ctx, 20, time.Second)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer d.Close()

	migrationsDir := "migrations"
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		migrationsDir = v
	}
	if err := d.Migrate(ctx, migrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	hub := ws.NewHub()
	srv := api.New(d, hub, ai.New(), mail.FromEnv())
	srv.Images = images.New(d.Pool)

	// Prometheus metrics. Gauges that depend on live state are pulled on
	// scrape via closures so the metrics package doesn't import api/ws/db.
	mx := metrics.New(metrics.Options{
		OnlinePlayers: func() int {
			n := 0
			for _, c := range hub.OnlinePlayerCounts() {
				n += c
			}
			return n
		},
		GamesByState: func() map[string]int {
			out := map[string]int{"setup": 0, "game": 0, "finished": 0}
			gctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			gs, err := d.ListGames(gctx)
			if err != nil {
				return out
			}
			for _, g := range gs {
				out[g.State]++
			}
			return out
		},
	})
	srv.Metrics = mx

	srv.ResumeAutoCloseTimers(ctx)

	gcCtx, cancelGC := context.WithCancel(ctx)
	defer cancelGC()
	go srv.RunOrphanImageGC(gcCtx)

	corsMW := cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Player-Token"},
		AllowCredentials: false,
	})

	root := http.NewServeMux()
	// /metrics and /health are mounted at the root mux, outside the CORS and
	// request-log wrappers, so scrapes/probes don't pollute access logs and
	// /metrics isn't CORS-exposed to browsers. /metrics checks its token inside
	// the handler.
	root.Handle("/metrics", mx.Handler(os.Getenv("METRICS_TOKEN")))
	root.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	root.Handle("/", corsMW(middleware.Logger(srv.Routes())))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           root,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func connectWithRetry(ctx context.Context, attempts int, wait time.Duration) (*db.DB, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		d, err := db.Connect(ctx)
		if err == nil {
			return d, nil
		}
		lastErr = err
		log.Printf("db not ready (%d/%d): %v", i+1, attempts, err)
		time.Sleep(wait)
	}
	return nil, lastErr
}
