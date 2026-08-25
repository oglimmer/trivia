package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/game"
)

// identityShuffle keeps import order stable so assertions can name positions.
func identityShuffle(int, func(i, j int)) {}

const twoQuestionImport = `{"questions":[
  {"text":"Name something you always forget to pack.","answers":[
    {"text":"Toothbrush","points":41},{"text":"Charger","points":22},
    {"text":"Socks","points":11},{"text":"Sunscreen","points":7},{"text":"Passport","points":4}]},
  {"text":"Name a bad excuse for being late.","answers":[
    {"text":"Traffic","points":38},{"text":"Alarm","points":25},
    {"text":"Dog","points":12},{"text":"Train","points":9},{"text":"Weather","points":6}]}
]}`

func pollGame(t *testing.T, f *fakeStore) *db.Game {
	t.Helper()
	g, err := f.CreateGame(context.TODO(), "consensus", "IRL Dubrovnik", 90, nil, "poll")
	if err != nil {
		t.Fatalf("create poll game: %v", err)
	}
	return g
}

func TestImportCreatesAuthorlessPollQuestions(t *testing.T) {
	s, f := testServer(t)
	pollGame(t, f)

	rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/import",
		body: twoQuestionImport, bearer: adminBearer(t)})
	if rec.Code != http.StatusOK {
		t.Fatalf("import: got %d — %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Imported  int           `json:"imported"`
		Questions []db.Question `json:"questions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Imported != 2 {
		t.Fatalf("imported %d questions, want 2", out.Imported)
	}
	for i, q := range out.Questions {
		if q.AnswerType != "poll" {
			t.Errorf("question %d: answerType %q, want poll", i, q.AnswerType)
		}
		if q.UserID != "" {
			t.Errorf("question %d: expected no author, got %q", i, q.UserID)
		}
		if n := len(game.ParsePollOptions(q.Options)); n != 5 {
			t.Errorf("question %d: %d options, want 5", i, n)
		}
		if q.SortOrder != i+1 {
			t.Errorf("question %d: sortOrder %d, want %d", i, q.SortOrder, i+1)
		}
	}
}

// Re-importing after fixing a typo in the points is the expected workflow. It
// must replace the set, not append to it.
func TestReimportReplacesRatherThanAppends(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	for i := 0; i < 3; i++ {
		rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/import",
			body: twoQuestionImport, bearer: adminBearer(t)})
		if rec.Code != http.StatusOK {
			t.Fatalf("import %d: got %d — %s", i, rec.Code, rec.Body.String())
		}
	}
	qs, err := f.ListQuestions(context.TODO(), g.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("after 3 imports there are %d questions, want 2", len(qs))
	}
}

func TestImportRejectsBadPayloads(t *testing.T) {
	cases := map[string]string{
		"no questions":     `{"questions":[]}`,
		"four answers":     `{"questions":[{"text":"Q","answers":[{"text":"a","points":1},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1}]}]}`,
		"blank text":       `{"questions":[{"text":"  ","answers":[{"text":"a","points":1},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}]}`,
		"negative points":  `{"questions":[{"text":"Q","answers":[{"text":"a","points":-1},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}]}`,
		"duplicate answer": `{"questions":[{"text":"Q","answers":[{"text":"Pizza","points":5},{"text":"pizza","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, f := testServer(t)
			pollGame(t, f)
			rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/import",
				body: body, bearer: adminBearer(t)})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 — %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestImportRefusedOnClassicGameAndAfterStart(t *testing.T) {
	s, f := testServer(t)
	if _, err := f.CreateGame(context.TODO(), "abcd", "Vienna", 30, nil, "classic"); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s, req{method: "POST", path: "/api/admin/games/abcd/questions/import",
		body: twoQuestionImport, bearer: adminBearer(t)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("classic game: got %d, want 400", rec.Code)
	}

	g := pollGame(t, f)
	if err := f.SetGameState(context.TODO(), g.ID, "game"); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/import",
		body: twoQuestionImport, bearer: adminBearer(t)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("running game: got %d, want 400", rec.Code)
	}
}

// Without the shuffle the top survey answer always sits in slot 0 and the whole
// game is won by tapping the first row every time.
func TestImportShufflesOptions(t *testing.T) {
	body := importQuestionsBody{}
	if err := json.Unmarshal([]byte(twoQuestionImport), &body); err != nil {
		t.Fatal(err)
	}
	// A reversing "shuffle" proves toHostQuestions actually applies the hook.
	reverse := func(n int, swap func(i, j int)) {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			swap(i, j)
		}
	}
	items, err := body.toHostQuestions(reverse)
	if err != nil {
		t.Fatal(err)
	}
	opts := game.ParsePollOptions(items[0].Options)
	if opts[0].Text != "Passport" || opts[4].Text != "Toothbrush" {
		t.Fatalf("options were not passed through the shuffle: %+v", opts)
	}
}

// The points ARE the answer. If they reach a phone before the reveal, every
// team can read a perfect score off the network tab.
func TestPollPointsNeverReachPlayersBeforeReveal(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	if _, err := f.ReplaceHostQuestions(context.TODO(), g.ID, mustHostQuestions(t)); err != nil {
		t.Fatal(err)
	}
	u, err := f.CreateUser(context.TODO(), g.ID, "Team Rocket", nil, "", "tok-team")
	if err != nil {
		t.Fatal(err)
	}

	rec := do(t, s, req{method: "GET", path: "/api/games/consensus/questions", playerTo: u.Token})
	if rec.Code != http.StatusOK {
		t.Fatalf("list questions: got %d", rec.Code)
	}
	// Assert on the parsed options, not on a substring of the whole body: a
	// number like 41 turns up inside createdAt timestamps often enough to make
	// a text search a coin flip.
	var qs []db.Question
	if err := json.Unmarshal(rec.Body.Bytes(), &qs); err != nil {
		t.Fatal(err)
	}
	if len(qs) == 0 {
		t.Fatal("no questions returned")
	}
	for _, q := range qs {
		opts := game.ParsePollOptions(q.Options)
		if len(opts) != 5 {
			t.Fatalf("question %q: got %d options, want 5", q.Text, len(opts))
		}
		for _, o := range opts {
			if o.Points != 0 {
				t.Fatalf("point values leaked to a player before reveal: %q is worth %d", o.Text, o.Points)
			}
			// The answer text must still be there, or there is nothing to tap.
			if o.Text == "" {
				t.Fatalf("answer text missing from the player payload for %q", q.Text)
			}
		}
	}
}

func TestPollPointsVisibleOnceFinished(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	if _, err := f.ReplaceHostQuestions(context.TODO(), g.ID, mustHostQuestions(t)); err != nil {
		t.Fatal(err)
	}
	if err := f.SetGameState(context.TODO(), g.ID, "finished"); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s, req{method: "GET", path: "/api/games/consensus/questions"})
	var qs []db.Question
	if err := json.Unmarshal(rec.Body.Bytes(), &qs); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, q := range qs {
		for _, o := range game.ParsePollOptions(q.Options) {
			total += o.Points
		}
	}
	if total == 0 {
		t.Fatalf("points should be public once the game is finished:\n%s", rec.Body.String())
	}
}

func TestStripPollPointsLeavesOtherTypesAlone(t *testing.T) {
	opts := json.RawMessage(`["Vienna","Zagreb","Dubrovnik"]`)
	if got := stripPollPoints("choice", opts); string(got) != string(opts) {
		t.Errorf("choice options were altered: %s", got)
	}
}

func mustHostQuestions(t *testing.T) []db.HostQuestion {
	t.Helper()
	var body importQuestionsBody
	if err := json.Unmarshal([]byte(twoQuestionImport), &body); err != nil {
		t.Fatal(err)
	}
	items, err := body.toHostQuestions(identityShuffle)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// A poll game must show the standings all the way to the end — the TV board is
// the whole point of the format.
func TestPollGamesDisableTheLeaderboardSuspenseTail(t *testing.T) {
	g := &db.Game{State: "game", Mode: "poll", HideLeaderboardTail: false}
	if inLeaderboardSuspense(g, 15, 15) {
		t.Error("poll game hid the leaderboard on the final question")
	}
	classic := &db.Game{State: "game", Mode: "classic", HideLeaderboardTail: true}
	if !inLeaderboardSuspense(classic, 15, 15) {
		t.Error("classic game should still hide the tail")
	}
}

const oneQuestionBody = `{"text":"Name a bad excuse for being late.","answers":[
  {"text":"Traffic","points":38},{"text":"Alarm","points":25},
  {"text":"Dog","points":12},{"text":"Train","points":9},{"text":"Weather","points":6}]}`

func TestCreatePollQuestionAppendsToTheRunningOrder(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)

	for i := 0; i < 3; i++ {
		rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions",
			body: oneQuestionBody, bearer: adminBearer(t)})
		if rec.Code != http.StatusOK {
			t.Fatalf("create %d: got %d — %s", i, rec.Code, rec.Body.String())
		}
	}
	qs, err := f.ListQuestions(context.TODO(), g.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 3 {
		t.Fatalf("got %d questions, want 3", len(qs))
	}
	for i, q := range qs {
		if q.SortOrder != i+1 {
			t.Errorf("question %d has sortOrder %d, want %d", i, q.SortOrder, i+1)
		}
		if q.AnswerType != "poll" || q.UserID != "" {
			t.Errorf("question %d: got type %q author %q, want poll and no author", i, q.AnswerType, q.UserID)
		}
	}
}

func TestUpdatePollQuestionKeepsItsPosition(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	if _, err := f.ReplaceHostQuestions(context.TODO(), g.ID, mustHostQuestions(t)); err != nil {
		t.Fatal(err)
	}
	qs, _ := f.ListQuestions(context.TODO(), g.ID, true)
	target := qs[1]

	rec := do(t, s, req{method: "PUT", path: "/api/admin/games/consensus/questions/" + target.ID,
		body: `{"text":"Edited question","answers":[
			{"text":"One","points":50},{"text":"Two","points":20},
			{"text":"Three","points":15},{"text":"Four","points":10},{"text":"Five","points":5}]}`,
		bearer: adminBearer(t)})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: got %d — %s", rec.Code, rec.Body.String())
	}

	after, _ := f.ListQuestions(context.TODO(), g.ID, true)
	if len(after) != len(qs) {
		t.Fatalf("update changed the question count: %d → %d", len(qs), len(after))
	}
	if after[1].ID != target.ID {
		t.Errorf("question moved position on edit: %q is now at index 1", after[1].ID)
	}
	if after[1].Text != "Edited question" {
		t.Errorf("text not saved: %q", after[1].Text)
	}
	opts := game.ParsePollOptions(after[1].Options)
	if len(opts) != 5 {
		t.Fatalf("got %d options after edit, want 5", len(opts))
	}
}

func TestUpdateRefusesAPlayerWrittenQuestion(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	u, err := f.CreateUser(context.TODO(), g.ID, "Team Red", nil, "", "tok-red")
	if err != nil {
		t.Fatal(err)
	}
	// A player-authored question in the same game must not be editable through
	// the host's CRUD endpoints — it belongs to its author.
	q, err := f.UpsertQuestion(context.TODO(), g.ID, u.ID, "Mine", nil, "yesno",
		json.RawMessage(`[]`), json.RawMessage(`"yes"`))
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, s, req{method: "PUT", path: "/api/admin/games/consensus/questions/" + q.ID,
		body: oneQuestionBody, bearer: adminBearer(t)})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404 — %s", rec.Code, rec.Body.String())
	}
}

func TestMovePollQuestionReorders(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	if _, err := f.ReplaceHostQuestions(context.TODO(), g.ID, mustHostQuestions(t)); err != nil {
		t.Fatal(err)
	}
	before, _ := f.ListQuestions(context.TODO(), g.ID, true)
	second := before[1]

	rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/" + second.ID + "/move",
		body: `{"direction":"up"}`, bearer: adminBearer(t)})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("move up: got %d — %s", rec.Code, rec.Body.String())
	}
	after, _ := f.ListQuestions(context.TODO(), g.ID, true)
	if after[0].ID != second.ID {
		t.Errorf("expected %q to be first after moving up, got %q", second.ID, after[0].ID)
	}

	// Moving the first question up again is a no-op, not an error — the button
	// is simply disabled-equivalent at the boundary.
	rec = do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/" + second.ID + "/move",
		body: `{"direction":"up"}`, bearer: adminBearer(t)})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("move past the top: got %d, want 204", rec.Code)
	}
	stillFirst, _ := f.ListQuestions(context.TODO(), g.ID, true)
	if stillFirst[0].ID != second.ID {
		t.Errorf("no-op move reordered the set")
	}
}

func TestMoveRejectsABadDirection(t *testing.T) {
	s, f := testServer(t)
	g := pollGame(t, f)
	if _, err := f.ReplaceHostQuestions(context.TODO(), g.ID, mustHostQuestions(t)); err != nil {
		t.Fatal(err)
	}
	qs, _ := f.ListQuestions(context.TODO(), g.ID, true)
	rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions/" + qs[0].ID + "/move",
		body: `{"direction":"sideways"}`, bearer: adminBearer(t)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestPollCrudRefusedOnClassicGameAndAfterStart(t *testing.T) {
	s, f := testServer(t)
	if _, err := f.CreateGame(context.TODO(), "abcd", "Vienna", 30, nil, "classic"); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s, req{method: "POST", path: "/api/admin/games/abcd/questions",
		body: oneQuestionBody, bearer: adminBearer(t)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("classic game: got %d, want 400", rec.Code)
	}

	g := pollGame(t, f)
	if err := f.SetGameState(context.TODO(), g.ID, "game"); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions",
		body: oneQuestionBody, bearer: adminBearer(t)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("running game: got %d, want 400", rec.Code)
	}
}

func TestCreateRejectsBadQuestions(t *testing.T) {
	cases := map[string]string{
		"blank text":      `{"text":"   ","answers":[{"text":"a","points":1},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}`,
		"three answers":   `{"text":"Q","answers":[{"text":"a","points":1},{"text":"b","points":1},{"text":"c","points":1}]}`,
		"blank answer":    `{"text":"Q","answers":[{"text":"","points":1},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}`,
		"negative points": `{"text":"Q","answers":[{"text":"a","points":-5},{"text":"b","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}`,
		"duplicate":       `{"text":"Q","answers":[{"text":"Pizza","points":5},{"text":"PIZZA","points":1},{"text":"c","points":1},{"text":"d","points":1},{"text":"e","points":1}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, f := testServer(t)
			pollGame(t, f)
			rec := do(t, s, req{method: "POST", path: "/api/admin/games/consensus/questions",
				body: body, bearer: adminBearer(t)})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 — %s", rec.Code, rec.Body.String())
			}
		})
	}
}
