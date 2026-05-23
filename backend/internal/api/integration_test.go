//go:build integration

// Integration test: real Postgres (via testcontainers) + real WebSocket hub +
// real scoring + real reveal. Opt in with `go test -tags=integration ./...`.
// Requires a running Docker daemon.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	gwebsocket "github.com/gorilla/websocket"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/images"
	"github.com/oglimmer/trivia/backend/internal/mail"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// TestIntegration_GameFlow drives a full game (create → join → submit
// questions → activate → answer over WS → reveal) against a real Postgres
// container and the real WebSocket hub, asserting that scoring lands on the
// leaderboard correctly for both immediate scoring (yesno) and the
// reveal-time rescore path (number).
func TestIntegration_GameFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	d := connectDB(t, ctx, pg)

	hub := ws.NewHub()
	srv := New(d, hub, &ai.Client{}, &mail.Mailer{})
	imgSvc := images.New(d.Pool)
	srv.Images = imgSvc

	httpSrv := httptest.NewServer(srv.Routes())
	t.Cleanup(httpSrv.Close)

	// Seed an image for every question (putQuestion requires photoImageId).
	imgYes := storePNGImage(t, ctx, imgSvc, 64, 48)
	imgNum := storePNGImage(t, ctx, imgSvc, 64, 48)

	// ---- admin login + create game ----
	var login struct{ Token string }
	doJSON(t, "POST", httpSrv.URL+"/api/admin/login", nil, `{"password":"letmein"}`, &login)
	if login.Token == "" {
		t.Fatal("admin login returned empty token")
	}
	adminHdr := http.Header{"Authorization": {"Bearer " + login.Token}}

	var game db.Game
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games", adminHdr,
		`{"name":"Integration","questionTimeoutSeconds":30}`, &game)
	if game.Code == "" || game.ID == "" {
		t.Fatalf("createGame returned %+v", game)
	}

	// ---- two players join ----
	type joinResp struct{ Token, UserID, GameID, Code string }
	var alice, bob joinResp
	doJSON(t, "POST", httpSrv.URL+"/api/games/"+game.Code+"/join", nil, `{"name":"Alice"}`, &alice)
	doJSON(t, "POST", httpSrv.URL+"/api/games/"+game.Code+"/join", nil, `{"name":"Bob"}`, &bob)

	// ---- each player submits one question (yesno + number) ----
	aliceHdr := http.Header{"X-Player-Token": {alice.Token}}
	bobHdr := http.Header{"X-Player-Token": {bob.Token}}

	var qYes db.Question
	body := fmt.Sprintf(`{"text":"Sky blue?","photoImageId":%q,"answerType":"yesno","options":[],"correct":"yes"}`, imgYes)
	doJSON(t, "PUT", httpSrv.URL+"/api/games/"+game.Code+"/questions", aliceHdr, body, &qYes)

	var qNum db.Question
	body = fmt.Sprintf(`{"text":"How many?","photoImageId":%q,"answerType":"number","options":[],"correct":100}`, imgNum)
	doJSON(t, "PUT", httpSrv.URL+"/api/games/"+game.Code+"/questions", bobHdr, body, &qNum)

	// ---- transition to game; this shuffles question order ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/state", adminHdr,
		`{"state":"game"}`, http.StatusNoContent)

	// ---- open WebSockets ----
	aliceWS := dialWS(t, httpSrv.URL, "?token="+alice.Token)
	bobWS := dialWS(t, httpSrv.URL, "?token="+bob.Token)
	adminWS := dialWS(t, httpSrv.URL, "?role=admin&token="+login.Token+"&code="+game.Code)

	// Drain the initial frames each client gets on join: gameState, users,
	// presence (admin only), questionsAdmin (admin only).
	for _, c := range []*wsClient{aliceWS, bobWS, adminWS} {
		_ = c.waitFor(t, 5*time.Second, "gameState")
	}

	// ---- run question 1: whichever is activated first ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/activate", adminHdr,
		`{}`, http.StatusNoContent)

	firstState := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active"
	})
	firstQID := stringField(firstState, "currentQuestionId")
	if firstQID == "" {
		t.Fatalf("admin gameState missing currentQuestionId: %v", firstState)
	}
	firstType := nestedString(firstState, "question", "answerType")

	// Wait for the players to see the same active question before answering;
	// otherwise the answer races the state transition.
	aliceWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == firstQID
	})
	bobWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == firstQID
	})

	// Pause so responseMs > 0 and the time bonus calculation actually runs.
	time.Sleep(80 * time.Millisecond)

	aliceVal, bobVal, aliceShouldWin := answerValuesFor(firstType)
	aliceWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": firstQID, "value": aliceVal},
	})
	bobWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": firstQID, "value": bobVal},
	})
	aliceWS.waitFor(t, 5*time.Second, "answerAck")
	bobWS.waitFor(t, 5*time.Second, "answerAck")

	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/reveal", adminHdr,
		"", http.StatusNoContent)

	revealed1 := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "revealed" && stringField(d, "currentQuestionId") == firstQID
	})
	scores1 := scoresByName(t, revealed1)
	// At least one player must have scored: scoring lands in real Postgres and
	// surfaces on the leaderboard.
	if scores1["Alice"]+scores1["Bob"] == 0 {
		t.Fatalf("expected at least one positive score after reveal, got %v", scores1)
	}
	if aliceShouldWin && scores1["Alice"] <= scores1["Bob"] {
		t.Errorf("first question: Alice should outscore Bob, got %v", scores1)
	}

	// ---- next question ----
	var nextResp struct {
		Done       bool
		QuestionID string
	}
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/next", adminHdr, "", &nextResp)
	if nextResp.Done {
		t.Fatal("expected another question, got done=true")
	}
	secondQID := nextResp.QuestionID
	if secondQID == "" || secondQID == firstQID {
		t.Fatalf("expected fresh question id, got %q (first was %q)", secondQID, firstQID)
	}

	secondState := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == secondQID
	})
	secondType := nestedString(secondState, "question", "answerType")

	aliceWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == secondQID
	})
	bobWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == secondQID
	})

	time.Sleep(80 * time.Millisecond)

	aliceVal2, bobVal2, aliceShouldWin2 := answerValuesFor(secondType)
	aliceWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": secondQID, "value": aliceVal2},
	})
	bobWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": secondQID, "value": bobVal2},
	})
	aliceWS.waitFor(t, 5*time.Second, "answerAck")
	bobWS.waitFor(t, 5*time.Second, "answerAck")

	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/reveal", adminHdr,
		"", http.StatusNoContent)

	revealed2 := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "revealed" && stringField(d, "currentQuestionId") == secondQID
	})
	scores2 := scoresByName(t, revealed2)
	if aliceShouldWin2 && scores2["Alice"] <= scores2["Bob"] {
		t.Errorf("second question: Alice should outscore Bob, got %v", scores2)
	}

	// ---- final next → game finishes ----
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/next", adminHdr, "", &nextResp)
	if !nextResp.Done {
		t.Fatalf("expected done=true at end of game, got %+v", nextResp)
	}

	var finalState struct {
		State string `json:"state"`
		Code  string `json:"code"`
	}
	doJSON(t, "GET", httpSrv.URL+"/api/games/"+game.Code, nil, "", &finalState)
	if finalState.State != "finished" {
		t.Errorf("game state: want finished, got %q", finalState.State)
	}
}

// answerValuesFor returns (alice, bob, aliceShouldWin) values for the given
// question type. Alice is set up to win on both yesno and number — the test
// then asserts that her leaderboard total reflects this.
func answerValuesFor(answerType string) (any, any, bool) {
	switch answerType {
	case "yesno":
		// qYes has correct="yes". Alice picks yes; Bob picks no.
		return "yes", "no", true
	case "number":
		// qNum has correct=100. Alice guesses 101 (within tolerance → full
		// base + time bonus), Bob guesses 150 (rank-scored, much further).
		return 101, 150, true
	default:
		return nil, nil, false
	}
}

// ---------- testcontainers + db plumbing ----------

func startPostgres(t *testing.T, ctx context.Context) *tcpg.PostgresContainer {
	t.Helper()
	pg, err := tcpg.Run(ctx, "postgres:16-alpine",
		tcpg.WithDatabase("trivia"),
		tcpg.WithUsername("trivia"),
		tcpg.WithPassword("trivia"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v (is Docker running?)", err)
	}
	t.Cleanup(func() {
		// Use a fresh context: the test's ctx may already be cancelled.
		_ = pg.Terminate(context.Background())
	})
	return pg
}

func connectDB(t *testing.T, ctx context.Context, pg *tcpg.PostgresContainer) *db.DB {
	t.Helper()
	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	t.Setenv("POSTGRES_HOST", host)
	t.Setenv("POSTGRES_PORT", port.Port())
	t.Setenv("POSTGRES_USER", "trivia")
	t.Setenv("POSTGRES_PASSWORD", "trivia")
	t.Setenv("POSTGRES_DB", "trivia")

	d, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	t.Cleanup(d.Close)

	// Resolve migrations directory relative to this test file, not CWD, so
	// `go test ./...` from the repo root works the same as `go test` in
	// internal/api.
	_, thisFile, _, _ := runtime.Caller(0)
	migrations := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
	if err := d.Migrate(ctx, migrations); err != nil {
		t.Fatalf("migrate %s: %v", migrations, err)
	}
	return d
}

// storePNGImage encodes a tiny gradient PNG and runs it through the real
// images.Service so a photoImageId exists in Postgres.
func storePNGImage(t *testing.T, ctx context.Context, svc *images.Service, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Vary per call to defeat dedupe — two questions need two distinct
			// image rows, otherwise the photoImageId returned would alias.
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8((x*y + w + h) & 0xff), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	id, err := svc.Store(ctx, &buf)
	if err != nil {
		t.Fatalf("store image: %v", err)
	}
	return id
}

// ---------- HTTP helpers ----------

func doJSON(t *testing.T, method, url string, headers http.Header, body string, out any) {
	t.Helper()
	resp := doRequest(t, method, url, headers, body)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d, body %s", method, url, resp.StatusCode, b)
	}
	if out == nil || len(b) == 0 {
		return
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("decode %s response: %v (body %s)", url, err, b)
	}
}

func doStatus(t *testing.T, method, url string, headers http.Header, body string, want int) {
	t.Helper()
	resp := doRequest(t, method, url, headers, body)
	defer resp.Body.Close()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status %d (want %d), body %s", method, url, resp.StatusCode, want, b)
	}
}

func doRequest(t *testing.T, method, url string, headers http.Header, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// ---------- WS helpers ----------

// wsClient wraps a gorilla websocket connection with a background reader that
// pushes JSON frames onto an unbuffered-friendly channel so the test can wait
// for specific message types with a timeout.
type wsClient struct {
	conn *gwebsocket.Conn
	msgs chan map[string]any
	name string
}

func dialWS(t *testing.T, baseURL, query string) *wsClient {
	t.Helper()
	wsAddr := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws" + query
	conn, _, err := gwebsocket.DefaultDialer.Dial(wsAddr, nil)
	if err != nil {
		t.Fatalf("ws dial %s: %v", wsAddr, err)
	}
	c := &wsClient{
		conn: conn,
		msgs: make(chan map[string]any, 64),
		name: query,
	}
	go func() {
		defer close(c.msgs)
		for {
			var m map[string]any
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			c.msgs <- m
		}
	}()
	t.Cleanup(func() { _ = conn.Close() })
	return c
}

func (c *wsClient) sendJSON(t *testing.T, v any) {
	t.Helper()
	if err := c.conn.WriteJSON(v); err != nil {
		t.Fatalf("ws %s write: %v", c.name, err)
	}
}

// waitFor returns the next message whose "type" is in want. Any frame with a
// non-matching type (e.g. presence) is silently discarded.
func (c *wsClient) waitFor(t *testing.T, timeout time.Duration, want ...string) map[string]any {
	t.Helper()
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-c.msgs:
			if !ok {
				t.Fatalf("ws %s closed while waiting for %v", c.name, want)
				return nil
			}
			if wantSet[stringField(m, "type")] {
				return m
			}
		case <-deadline:
			t.Fatalf("ws %s: timeout waiting for %v", c.name, want)
			return nil
		}
	}
}

// waitForGameStateWhere returns the data block of the next "gameState" frame
// whose data matches pred. Earlier gameStates that don't match are skipped —
// this is how the test waits for a *new* state after triggering a transition.
func (c *wsClient) waitForGameStateWhere(t *testing.T, timeout time.Duration, pred func(data map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-c.msgs:
			if !ok {
				t.Fatalf("ws %s closed while waiting for matching gameState", c.name)
				return nil
			}
			if stringField(m, "type") != "gameState" {
				continue
			}
			d, _ := m["data"].(map[string]any)
			if d != nil && pred(d) {
				return d
			}
		case <-deadline:
			t.Fatalf("ws %s: timeout waiting for matching gameState", c.name)
			return nil
		}
	}
}

// ---------- small JSON walkers ----------

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func nestedString(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			return stringField(cur, k)
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

// scoresByName plucks the leaderboard from a gameState data block and returns
// {userName: points}. Fails the test if the leaderboard is missing or empty.
func scoresByName(t *testing.T, data map[string]any) map[string]int {
	t.Helper()
	lb, ok := data["leaderboard"].([]any)
	if !ok {
		t.Fatalf("gameState missing leaderboard: %v", data)
	}
	out := map[string]int{}
	for _, e := range lb {
		row, _ := e.(map[string]any)
		name := stringField(row, "userName")
		pts, _ := row["points"].(float64)
		out[name] = int(pts)
	}
	return out
}
