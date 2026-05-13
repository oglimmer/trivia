package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/auth"
	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// testServer builds a Server wired to fresh fake state.
func testServer(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	f := newFakeStore()
	s := New(f, ws.NewHub(), &ai.Client{})
	return s, f
}

// adminBearer returns a valid admin Authorization header.
func adminBearer(t *testing.T) string {
	t.Helper()
	tok, err := auth.Issue("admin", "admin", time.Minute)
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	return "Bearer " + tok
}

type req struct {
	method   string
	path     string
	body     string
	bearer   string
	playerTo string
}

func do(t *testing.T, s *Server, r req) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if r.body != "" {
		bodyReader = strings.NewReader(r.body)
	}
	httpReq := httptest.NewRequest(r.method, r.path, bodyReader)
	if r.body != "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if r.bearer != "" {
		httpReq.Header.Set("Authorization", r.bearer)
	}
	if r.playerTo != "" {
		httpReq.Header.Set("X-Player-Token", r.playerTo)
	}
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httpReq)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return v
}

// ---------- admin login ----------

func TestAdminLoginWrongPassword(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "POST", path: "/api/admin/login", body: `{"password":"nope"}`})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAdminLoginAccepts(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "POST", path: "/api/admin/login", body: `{"password":"letmein"}`})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	out := decode[map[string]string](t, w)
	if out["token"] == "" {
		t.Fatalf("expected token in response")
	}
}

func TestAdminEndpointRequiresAuth(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "GET", path: "/api/admin/games"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// ---------- create / list games ----------

func TestCreateGameClampsTimeout(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games",
		bearer: adminBearer(t),
		body:   `{"code":"ABC","name":"Quiz","questionTimeoutSeconds":10000}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	g := decode[db.Game](t, w)
	if g.QuestionTimeoutSeconds != maxQuestionTimeoutSeconds {
		t.Errorf("timeout: want %d, got %d", maxQuestionTimeoutSeconds, g.QuestionTimeoutSeconds)
	}
	if g.Code != "abc" {
		t.Errorf("code should be lowercased: got %q", g.Code)
	}
}

func TestCreateGameGeneratesCodeWhenEmpty(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games",
		bearer: adminBearer(t),
		body:   `{"name":"Quiz"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	g := decode[db.Game](t, w)
	if len(g.Code) != 4 {
		t.Errorf("expected 4-char generated code, got %q", g.Code)
	}
}

func TestListGamesEmpty(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "GET", path: "/api/admin/games", bearer: adminBearer(t)})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("want empty array, got %s", w.Body.String())
	}
}

// ---------- player join ----------

func TestGetGameForJoin404(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "GET", path: "/api/games/missing"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestGetGameForJoinHappy(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	out := decode[map[string]any](t, w)
	if out["state"] != "setup" || out["code"] != "abcd" {
		t.Errorf("unexpected response: %v", out)
	}
}

func TestJoinGameRequiresName(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"   "}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinGameOnlyAllowedInSetup(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	_ = f.SetGameState(nil, g.ID, "game")
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice"}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (game not in setup), got %d", w.Code)
	}
}

func TestJoinGameHappy(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice","photoB64":"x"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	out := decode[map[string]string](t, w)
	if out["token"] == "" || out["userId"] == "" {
		t.Errorf("expected token and userId, got %v", out)
	}
}

// ---------- me ----------

func TestMeRequiresPlayerToken(t *testing.T) {
	s, _ := testServer(t)
	w := do(t, s, req{method: "GET", path: "/api/me"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestMeReturnsUserAndGame(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	u, _ := f.CreateUser(nil, g.ID, "Alice", "", "tok-1")
	w := do(t, s, req{method: "GET", path: "/api/me", playerTo: u.Token})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var out struct {
		User *db.User `json:"user"`
		Game *db.Game `json:"game"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.User == nil || out.User.Name != "Alice" {
		t.Errorf("unexpected user: %+v", out.User)
	}
	if out.Game == nil || out.Game.Code != "abcd" {
		t.Errorf("unexpected game: %+v", out.Game)
	}
}

// ---------- game state transitions ----------

func TestSetGameStateRejectsBad(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/state",
		bearer: adminBearer(t),
		body:   `{"state":"banana"}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSetGameStateToGameShufflesAndClears(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	// Put two questions on the game. SortOrder starts at 0 for both via Upsert.
	_, _ = f.UpsertQuestion(nil, g.ID, "u-a", "q1?", "x", "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_, _ = f.UpsertQuestion(nil, g.ID, "u-b", "q2?", "x", "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"no"`))
	// Pretend there's a current question, so we can verify it's cleared.
	qID := "q-stale"
	g.CurrentQuestionID = &qID
	_ = f.SetGameState(nil, g.ID, "setup") // no-op, just to ensure state is sane

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/state",
		bearer: adminBearer(t),
		body:   `{"state":"game"}`,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}
	updated, _ := f.GameByID(nil, g.ID)
	if updated.State != "game" {
		t.Errorf("state: want game, got %q", updated.State)
	}
	if updated.CurrentQuestionID != nil {
		t.Errorf("current question should be cleared, got %v", updated.CurrentQuestionID)
	}
	qs, _ := f.ListQuestions(nil, g.ID, true)
	if len(qs) != 2 {
		t.Fatalf("want 2 questions, got %d", len(qs))
	}
	// RandomizeQuestionOrder assigns sort_order 1..N. The default Upsert
	// leaves them at 0; verify they've been reassigned.
	for _, q := range qs {
		if q.SortOrder == 0 {
			t.Errorf("question %s still has sort_order=0; randomize did not run", q.ID)
		}
	}
}

// ---------- reveal / next ----------

func TestRevealRequiresActiveQuestion(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/reveal",
		bearer: adminBearer(t),
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRevealRescoresNumberAnswers(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	// One number question, three players with varying closeness.
	q, _ := f.UpsertQuestion(nil, g.ID, "author", "How many?", "x", "number",
		json.RawMessage(`[]`), json.RawMessage(`100`))
	for _, p := range []struct {
		uid    string
		answer string
		ms     int
	}{
		{"closest", `101`, 1000},
		{"mid", `108`, 1500},
		{"far", `130`, 2000},
	} {
		_ = f.SaveAnswer(nil, q.ID, p.uid, json.RawMessage(p.answer), p.ms, false, 0)
	}
	_ = f.ActivateQuestion(nil, g.ID, q.ID)

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/reveal",
		bearer: adminBearer(t),
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}
	updated, _ := f.GameByID(nil, g.ID)
	if updated.QuestionState != "revealed" {
		t.Errorf("question state: want revealed, got %q", updated.QuestionState)
	}
	ans, _ := f.AnswersForQuestion(nil, q.ID)
	byUser := map[string]db.Answer{}
	for _, a := range ans {
		byUser[a.UserID] = a
	}
	if byUser["closest"].Points <= byUser["mid"].Points {
		t.Errorf("closer guess must score higher: closest=%d mid=%d",
			byUser["closest"].Points, byUser["mid"].Points)
	}
}

func TestNextQuestionFinishesAtEnd(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	q, _ := f.UpsertQuestion(nil, g.ID, "author", "only", "x", "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_ = f.SetGameState(nil, g.ID, "game")
	_ = f.ActivateQuestion(nil, g.ID, q.ID)

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/next",
		bearer: adminBearer(t),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	out := decode[map[string]any](t, w)
	if out["done"] != true {
		t.Errorf("want done=true, got %v", out)
	}
	updated, _ := f.GameByID(nil, g.ID)
	if updated.State != "finished" {
		t.Errorf("game should be finished, got state %q", updated.State)
	}
}

// ---------- delete game cleanup ----------

func TestDeleteGameCancelsTimerAndDropsLock(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(nil, "abcd", "Quiz", 30)
	// Force lifecycle state populated.
	s.lockFor(g.ID)                                // populate gameLocks
	s.scheduleAutoClose(g.ID, "q-x", 24*time.Hour) // populate autoClose
	w := do(t, s, req{
		method: "DELETE",
		path:   "/api/admin/games/" + g.Code,
		bearer: adminBearer(t),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	s.mu.Lock()
	_, hasLock := s.gameLocks[g.ID]
	s.mu.Unlock()
	if hasLock {
		t.Errorf("expected gameLocks entry to be dropped")
	}
	s.autoCloseMu.Lock()
	_, hasTimer := s.autoClose[g.ID]
	s.autoCloseMu.Unlock()
	if hasTimer {
		t.Errorf("expected auto-close timer to be cancelled")
	}
}
