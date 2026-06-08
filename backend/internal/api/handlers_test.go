package api

import (
	"context"
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
	"github.com/oglimmer/trivia/backend/internal/mail"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// testServer builds a Server wired to fresh fake state.
func testServer(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	f := newFakeStore()
	s := New(f, ws.NewHub(), &ai.Client{}, &mail.Mailer{})
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"   "}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinGameAllowedMidGame(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_ = f.SetGameState(context.TODO(), g.ID, "game")
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (join allowed mid-game), got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinGameRejectedWhenFinished(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_ = f.SetGameState(context.TODO(), g.ID, "finished")
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice"}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (game finished), got %d", w.Code)
	}
}

func TestJoinGameHappy(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	out := decode[map[string]string](t, w)
	if out["token"] == "" || out["userId"] == "" {
		t.Errorf("expected token and userId, got %v", out)
	}
}

func TestJoinGameWithPhotoImageID(t *testing.T) {
	s, f := testServer(t)
	s.Images = newFakeImageStore("img-1")
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice","photoImageId":"img-1"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	users, _ := f.ListUsers(context.TODO(), g.ID)
	if len(users) != 1 {
		t.Fatalf("want 1 user, got %d", len(users))
	}
	if users[0].PhotoImageID == nil || *users[0].PhotoImageID != "img-1" {
		t.Errorf("photoImageId not stored: %+v", users[0].PhotoImageID)
	}
}

func TestJoinGameRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_, _ = f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-existing")
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"  alice  "}`,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 (name taken), got %d (%s)", w.Code, w.Body.String())
	}
	out := decode[map[string]string](t, w)
	if !strings.Contains(strings.ToLower(out["error"]), "already taken") {
		t.Errorf("expected user-friendly 'already taken' error, got %q", out["error"])
	}
}

func TestJoinGameAllowsSameNameInDifferentGames(t *testing.T) {
	s, f := testServer(t)
	g1, _ := f.CreateGame(context.TODO(), "aaaa", "Quiz A", 30, nil)
	g2, _ := f.CreateGame(context.TODO(), "bbbb", "Quiz B", 30, nil)
	_, _ = f.CreateUser(context.TODO(), g1.ID, "Alice", nil, "", "tok-g1")
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g2.Code + "/join",
		body:   `{"name":"Alice"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (different game), got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUpdateMeRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_, _ = f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-alice")
	bob, _ := f.CreateUser(context.TODO(), g.ID, "Bob", nil, "", "tok-bob")
	w := do(t, s, req{
		method:   "PUT",
		path:     "/api/me",
		body:     `{"name":"ALICE"}`,
		playerTo: bob.Token,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 (name taken), got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUpdateMeAllowsSameNameForSelf(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-alice")
	w := do(t, s, req{
		method:   "PUT",
		path:     "/api/me",
		body:     `{"name":"Alice"}`,
		playerTo: alice.Token,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204 (self-rename no-op), got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinGameRejectsUnknownPhotoImageID(t *testing.T) {
	s, f := testServer(t)
	s.Images = newFakeImageStore() // empty
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	w := do(t, s, req{
		method: "POST",
		path:   "/api/games/" + g.Code + "/join",
		body:   `{"name":"Alice","photoImageId":"missing"}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown image id, got %d (%s)", w.Code, w.Body.String())
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	u, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-1")
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

// ---------- putQuestion + leaderboard photo wiring ----------

func TestPutQuestionWithPhotoImageID(t *testing.T) {
	s, f := testServer(t)
	s.Images = newFakeImageStore("img-q")
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	u, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-author")

	body := `{"text":"real?","photoImageId":"img-q","answerType":"yesno","options":[],"correct":"yes"}`
	w := do(t, s, req{
		method:   "PUT",
		path:     "/api/games/" + g.Code + "/questions",
		body:     body,
		playerTo: u.Token,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	q := decode[db.Question](t, w)
	if q.PhotoImageID == nil || *q.PhotoImageID != "img-q" {
		t.Errorf("photoImageId not stored on question: %+v", q.PhotoImageID)
	}
}

func TestLeaderboardSurfacesPhotoImageID(t *testing.T) {
	s, f := testServer(t)
	s.Images = newFakeImageStore("img-le")
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	imgID := "img-le"
	_, _ = f.CreateUser(context.TODO(), g.ID, "Alice", &imgID, "", "tok-a")
	// Leaderboard is gated until the game is finished or a question is
	// revealed, so flip the game into a state where scores are exposed.
	_ = f.SetGameState(context.TODO(), g.ID, "finished")

	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/leaderboard"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	scores := decode[[]db.Score](t, w)
	if len(scores) != 1 {
		t.Fatalf("want 1 score row, got %d", len(scores))
	}
	if scores[0].PhotoImageID == nil || *scores[0].PhotoImageID != imgID {
		t.Errorf("photoImageId missing from leaderboard: %+v", scores[0].PhotoImageID)
	}
}

// TestLeaderboardHiddenUntilRevealed guards against the score-deduction leak:
// during an active (unrevealed) question a player who has answered must not be
// able to read points/correct from the public leaderboard endpoint and infer
// whether their answer was right.
func TestLeaderboardHiddenUntilRevealed(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_, _ = f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-a")

	// In-setup: must be empty.
	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/leaderboard"})
	if w.Code != http.StatusOK {
		t.Fatalf("setup: want 200, got %d", w.Code)
	}
	if got := decode[[]db.Score](t, w); len(got) != 0 {
		t.Errorf("setup: want empty leaderboard, got %+v", got)
	}

	// Active question: still must be empty (this is the leak that previously
	// existed — points/correct would tick up immediately on SaveAnswer).
	_ = f.SetGameState(context.TODO(), g.ID, "game")
	w = do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/leaderboard"})
	if got := decode[[]db.Score](t, w); len(got) != 0 {
		t.Errorf("active: want empty leaderboard, got %+v", got)
	}
}

// ---------- public question list: correct-answer visibility ----------

// TestListQuestionsPublicHidesOthersCorrectUntilFinished guards the fix from
// b389b67: before the game is finished, the public question list must expose a
// player's correct answer ONLY for their own question (so the editor can
// rehydrate it). Everyone else's correct answer must be stripped, and an
// anonymous caller must see none.
func TestListQuestionsPublicHidesOthersCorrectUntilFinished(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-alice")
	bob, _ := f.CreateUser(context.TODO(), g.ID, "Bob", nil, "", "tok-bob")

	qA, _ := f.UpsertQuestion(context.TODO(), g.ID, alice.ID, "Alice's?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	qB, _ := f.UpsertQuestion(context.TODO(), g.ID, bob.ID, "Bob's?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"no"`))

	// Game still in setup. Alice asks for the list: she sees her own correct
	// answer but not Bob's.
	w := do(t, s, req{
		method:   "GET",
		path:     "/api/games/" + g.Code + "/questions",
		playerTo: alice.Token,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	byID := map[string]db.Question{}
	for _, q := range decode[[]db.Question](t, w) {
		byID[q.ID] = q
	}
	if byID[qA.ID].Correct == nil {
		t.Errorf("Alice should see the correct answer for her own question")
	}
	if byID[qB.ID].Correct != nil {
		t.Errorf("Alice must NOT see Bob's correct answer before finish, got %s", byID[qB.ID].Correct)
	}

	// Anonymous caller (no player token) sees no correct answers at all.
	w = do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/questions"})
	for _, q := range decode[[]db.Question](t, w) {
		if q.Correct != nil {
			t.Errorf("anonymous caller leaked correct answer for %s: %s", q.ID, q.Correct)
		}
	}
}

// TestListQuestionsPublicExposesAllCorrectWhenFinished verifies that once the
// game is finished every correct answer is exposed, even to an anonymous
// caller (the results screen needs them).
func TestListQuestionsPublicExposesAllCorrectWhenFinished(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-alice")
	bob, _ := f.CreateUser(context.TODO(), g.ID, "Bob", nil, "", "tok-bob")
	_, _ = f.UpsertQuestion(context.TODO(), g.ID, alice.ID, "Alice's?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_, _ = f.UpsertQuestion(context.TODO(), g.ID, bob.ID, "Bob's?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"no"`))
	_ = f.SetGameState(context.TODO(), g.ID, "finished")

	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/questions"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	qs := decode[[]db.Question](t, w)
	if len(qs) != 2 {
		t.Fatalf("want 2 questions, got %d", len(qs))
	}
	for _, q := range qs {
		if q.Correct == nil {
			t.Errorf("finished game must expose correct answer for %s", q.ID)
		}
	}
}

// ---------- game state transitions ----------

func TestSetGameStateRejectsBad(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	// Put two questions on the game. SortOrder starts at 0 for both via Upsert.
	_, _ = f.UpsertQuestion(context.TODO(), g.ID, "u-a", "q1?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_, _ = f.UpsertQuestion(context.TODO(), g.ID, "u-b", "q2?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"no"`))
	// Pretend there's a current question, so we can verify it's cleared.
	qID := "q-stale"
	g.CurrentQuestionID = &qID
	_ = f.SetGameState(context.TODO(), g.ID, "setup") // no-op, just to ensure state is sane

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/state",
		bearer: adminBearer(t),
		body:   `{"state":"game"}`,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}
	updated, _ := f.GameByID(context.TODO(), g.ID)
	if updated.State != "game" {
		t.Errorf("state: want game, got %q", updated.State)
	}
	if updated.CurrentQuestionID != nil {
		t.Errorf("current question should be cleared, got %v", updated.CurrentQuestionID)
	}
	qs, _ := f.ListQuestions(context.TODO(), g.ID, true)
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

func TestSetGameStateToGamePrunesStalePlayers(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)

	now := time.Now()
	// Three users: one fresh (just created), one borderline-stale, one well-stale.
	// Touch them via the fake store so we can pin LastSeen exactly.
	fresh, _ := f.CreateUser(context.TODO(), g.ID, "Fresh", nil, "", "tok-fresh")
	stale, _ := f.CreateUser(context.TODO(), g.ID, "Stale", nil, "", "tok-stale")
	ancient, _ := f.CreateUser(context.TODO(), g.ID, "Ancient", nil, "", "tok-old")

	f.users[fresh.ID].LastSeen = now.Add(-1 * time.Minute)
	f.users[stale.ID].LastSeen = now.Add(-31 * time.Minute)
	f.users[ancient.ID].LastSeen = now.Add(-2 * time.Hour)

	// Stale player has authored a question — it must survive the cleanup with
	// user_id detached. We don't model the SET NULL in the fake; we just check
	// the question itself is still present.
	_, _ = f.UpsertQuestion(context.TODO(), g.ID, stale.ID, "stale question?", nil,
		"yesno", json.RawMessage(`[]`), json.RawMessage(`"yes"`))

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/state",
		bearer: adminBearer(t),
		body:   `{"state":"game"}`,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}

	users, _ := f.ListUsers(context.TODO(), g.ID)
	names := map[string]bool{}
	for _, u := range users {
		names[u.Name] = true
	}
	if !names["Fresh"] {
		t.Errorf("Fresh should still be present, got %v", names)
	}
	if names["Stale"] || names["Ancient"] {
		t.Errorf("stale players should be removed, got %v", names)
	}

	qs, _ := f.ListQuestions(context.TODO(), g.ID, true)
	if len(qs) != 1 {
		t.Fatalf("question authored by stale player should be retained, got %d", len(qs))
	}
}

// ---------- reveal / next ----------

func TestRevealRequiresActiveQuestion(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	// One number question, three players with varying closeness.
	q, _ := f.UpsertQuestion(context.TODO(), g.ID, "author", "How many?", nil, "number",
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
		_ = f.SaveAnswer(context.TODO(), q.ID, p.uid, json.RawMessage(p.answer), p.ms, false, 0)
	}
	_ = f.ActivateQuestion(context.TODO(), g.ID, q.ID)

	w := do(t, s, req{
		method: "POST",
		path:   "/api/admin/games/" + g.Code + "/reveal",
		bearer: adminBearer(t),
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}
	updated, _ := f.GameByID(context.TODO(), g.ID)
	if updated.QuestionState != "revealed" {
		t.Errorf("question state: want revealed, got %q", updated.QuestionState)
	}
	ans, _ := f.AnswersForQuestion(context.TODO(), q.ID)
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
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	q, _ := f.UpsertQuestion(context.TODO(), g.ID, "author", "only", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_ = f.SetGameState(context.TODO(), g.ID, "game")
	_ = f.ActivateQuestion(context.TODO(), g.ID, q.ID)

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
	updated, _ := f.GameByID(context.TODO(), g.ID)
	if updated.State != "finished" {
		t.Errorf("game should be finished, got state %q", updated.State)
	}
}

// ---------- delete game cleanup ----------

func TestResultsGatedOnFinished(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)

	// Game not finished — endpoint returns an empty slice, not the data.
	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/results"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := decode[[]questionResults](t, w); len(got) != 0 {
		t.Errorf("setup: want empty results, got %+v", got)
	}
}

func TestResultsBreakdown(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	_, _ = f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-a")
	_, _ = f.CreateUser(context.TODO(), g.ID, "Bob", nil, "", "tok-b")
	_, _ = f.CreateUser(context.TODO(), g.ID, "Cara", nil, "", "tok-c")

	// One choice question with 3 options; correct is index 1.
	qChoice, _ := f.UpsertQuestion(context.TODO(), g.ID, "author-1",
		"Which?", nil, "choice",
		json.RawMessage(`["A","B","C"]`), json.RawMessage(`1`))
	// Two players picked the correct option, one picked the wrong one.
	_ = f.SaveAnswer(context.TODO(), qChoice.ID, "user-1", json.RawMessage(`1`), 500, true, 100)
	_ = f.SaveAnswer(context.TODO(), qChoice.ID, "user-2", json.RawMessage(`1`), 800, true, 90)
	_ = f.SaveAnswer(context.TODO(), qChoice.ID, "user-3", json.RawMessage(`0`), 900, false, 0)

	// One yesno question — correct is "yes". Only two of three players answer.
	qYes, _ := f.UpsertQuestion(context.TODO(), g.ID, "author-2",
		"Real?", nil, "yesno", json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_ = f.SaveAnswer(context.TODO(), qYes.ID, "user-1", json.RawMessage(`"yes"`), 400, true, 100)
	_ = f.SaveAnswer(context.TODO(), qYes.ID, "user-2", json.RawMessage(`"no"`), 700, false, 0)

	_ = f.SetGameState(context.TODO(), g.ID, "finished")

	w := do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/results"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	got := decode[[]questionResults](t, w)
	if len(got) != 2 {
		t.Fatalf("want 2 questions, got %d", len(got))
	}

	byID := map[string]questionResults{}
	for _, r := range got {
		byID[r.QuestionID] = r
	}

	c := byID[qChoice.ID]
	if c.TotalPlayers != 3 || c.AnsweredCount != 3 || c.CorrectCount != 2 || c.IncorrectCount != 1 || c.NoAnswerCount != 0 {
		t.Errorf("choice tallies wrong: %+v", c)
	}
	if len(c.Distribution) != 3 {
		t.Fatalf("choice distribution: want 3 buckets, got %d", len(c.Distribution))
	}
	if c.Distribution[1].Count != 2 || !c.Distribution[1].IsCorrect {
		t.Errorf("choice correct-bucket wrong: %+v", c.Distribution[1])
	}
	if c.Distribution[0].Count != 1 || c.Distribution[0].IsCorrect {
		t.Errorf("choice wrong-bucket wrong: %+v", c.Distribution[0])
	}

	y := byID[qYes.ID]
	if y.AnsweredCount != 2 || y.NoAnswerCount != 1 {
		t.Errorf("yesno answered/noanswer wrong: %+v", y)
	}
	if len(y.Distribution) != 2 {
		t.Fatalf("yesno distribution: want 2 buckets, got %d", len(y.Distribution))
	}
	if y.Distribution[0].Label != "Yes" || y.Distribution[0].Count != 1 || !y.Distribution[0].IsCorrect {
		t.Errorf("yesno yes-bucket wrong: %+v", y.Distribution[0])
	}
	if y.Distribution[1].Label != "No" || y.Distribution[1].Count != 1 || y.Distribution[1].IsCorrect {
		t.Errorf("yesno no-bucket wrong: %+v", y.Distribution[1])
	}
}

func TestCastVoteRejectedBeforeFinished(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-a")
	q, _ := f.UpsertQuestion(context.TODO(), g.ID, alice.ID, "Q?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))

	w := do(t, s, req{
		method:   "POST",
		path:     "/api/games/" + g.Code + "/vote",
		body:     `{"questionId":"` + q.ID + `"}`,
		playerTo: alice.Token,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 before finished, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCastVoteIsFinalAndCounted(t *testing.T) {
	s, f := testServer(t)
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g.ID, "Alice", nil, "", "tok-a")
	bob, _ := f.CreateUser(context.TODO(), g.ID, "Bob", nil, "", "tok-b")
	q1, _ := f.UpsertQuestion(context.TODO(), g.ID, alice.ID, "Q1?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	q2, _ := f.UpsertQuestion(context.TODO(), g.ID, bob.ID, "Q2?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"no"`))
	_ = f.SetGameState(context.TODO(), g.ID, "finished")

	// Alice votes for q1.
	w := do(t, s, req{
		method:   "POST",
		path:     "/api/games/" + g.Code + "/vote",
		body:     `{"questionId":"` + q1.ID + `"}`,
		playerTo: alice.Token,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("first vote: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	first := decode[map[string]any](t, w)
	if first["questionId"] != q1.ID || first["cast"] != true {
		t.Fatalf("first vote response wrong: %+v", first)
	}

	// Alice tries to switch to q2 — votes are final, so this is a no-op that
	// returns her original pick and does NOT move the count.
	w = do(t, s, req{
		method:   "POST",
		path:     "/api/games/" + g.Code + "/vote",
		body:     `{"questionId":"` + q2.ID + `"}`,
		playerTo: alice.Token,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("second vote: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	second := decode[map[string]any](t, w)
	if second["questionId"] != q1.ID || second["cast"] != false {
		t.Fatalf("re-vote should be locked to q1 and not cast: %+v", second)
	}

	// myvote reflects Alice's locked pick.
	w = do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/myvote", playerTo: alice.Token})
	mine := decode[map[string]string](t, w)
	if mine["questionId"] != q1.ID {
		t.Errorf("myvote: want %q, got %q", q1.ID, mine["questionId"])
	}

	// Bob has not voted yet.
	w = do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/myvote", playerTo: bob.Token})
	if got := decode[map[string]string](t, w)["questionId"]; got != "" {
		t.Errorf("bob myvote: want empty, got %q", got)
	}

	// Vote tallies are admin-only: q1 has exactly one vote, q2 none.
	w = do(t, s, req{method: "GET", path: "/api/admin/games/" + g.Code + "/votes", bearer: adminBearer(t)})
	counts := decode[map[string]int](t, w)
	if counts[q1.ID] != 1 {
		t.Errorf("q1 votes: want 1, got %d", counts[q1.ID])
	}
	if counts[q2.ID] != 0 {
		t.Errorf("q2 votes: want 0, got %d", counts[q2.ID])
	}

	// The public results endpoint must NOT leak any vote tally, or the running
	// count would bias players still deciding their pick.
	w = do(t, s, req{method: "GET", path: "/api/games/" + g.Code + "/results"})
	for _, raw := range decode[[]map[string]any](t, w) {
		if _, ok := raw["voteCount"]; ok {
			t.Errorf("public results leaked voteCount: %+v", raw)
		}
	}
}

func TestCastVoteRejectsForeignQuestion(t *testing.T) {
	s, f := testServer(t)
	g1, _ := f.CreateGame(context.TODO(), "aaaa", "Quiz1", 30, nil)
	g2, _ := f.CreateGame(context.TODO(), "bbbb", "Quiz2", 30, nil)
	alice, _ := f.CreateUser(context.TODO(), g1.ID, "Alice", nil, "", "tok-a")
	// A question that belongs to a different game.
	other, _ := f.UpsertQuestion(context.TODO(), g2.ID, "author", "Q?", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	_ = f.SetGameState(context.TODO(), g1.ID, "finished")

	w := do(t, s, req{
		method:   "POST",
		path:     "/api/games/" + g1.Code + "/vote",
		body:     `{"questionId":"` + other.ID + `"}`,
		playerTo: alice.Token,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for foreign question, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDeleteGameCancelsTimerAndDropsLock(t *testing.T) {
	s, f := testServer(t)
	imgs := newFakeImageStore()
	s.Images = imgs
	g, _ := f.CreateGame(context.TODO(), "abcd", "Quiz", 30, nil)
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
	imgs.mu.Lock()
	calls := len(imgs.deleteOrphansCalls)
	imgs.mu.Unlock()
	if calls != 1 {
		t.Errorf("expected one orphan-image sweep, got %d", calls)
	}
}
