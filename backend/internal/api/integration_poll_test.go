//go:build integration

// Integration test for Company Consensus ("poll") mode: real Postgres via
// testcontainers, the real WebSocket hub, the real import path and the real
// scoring. Opt in with `go test -tags=integration ./...`.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/game"
	"github.com/oglimmer/trivia/backend/internal/images"
	"github.com/oglimmer/trivia/backend/internal/mail"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

const pollImportPayload = `{"questions":[
  {"text":"Name something you always forget to pack.","answers":[
    {"text":"Toothbrush","points":41},{"text":"Charger","points":22},
    {"text":"Socks","points":11},{"text":"Sunscreen","points":7},{"text":"Passport","points":4}]},
  {"text":"Name a bad excuse for being late.","answers":[
    {"text":"Traffic","points":38},{"text":"Alarm","points":25},
    {"text":"Dog","points":12},{"text":"Train","points":9},{"text":"Weather","points":6}]}
]}`

// TestIntegration_PollGameFlow runs the Dubrovnik format end to end: create a
// poll game, import a host-authored set, two teams join, each picks an option
// over the real WebSocket, and the survey points land on the leaderboard.
//
// It also asserts the two things that would quietly ruin the evening: that the
// point values never reach a team's socket before the reveal, and that the
// imported question order survives the setup→game transition.
func TestIntegration_PollGameFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	d := connectDB(t, ctx, pg)

	hub := ws.NewHub()
	srv := New(d, hub, &ai.Client{}, &mail.Mailer{})
	srv.Images = images.New(d.Pool)

	httpSrv := httptest.NewServer(srv.Routes())
	t.Cleanup(httpSrv.Close)

	var login struct{ Token string }
	doJSON(t, "POST", httpSrv.URL+"/api/admin/login", nil, `{"password":"letmein"}`, &login)
	adminHdr := http.Header{"Authorization": {"Bearer " + login.Token}}

	// ---- create a poll game ----
	var g db.Game
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games", adminHdr,
		`{"name":"IRL Dubrovnik","questionTimeoutSeconds":90,"mode":"poll"}`, &g)
	if g.Mode != "poll" {
		t.Fatalf("game mode is %q, want poll", g.Mode)
	}
	if g.HideLeaderboardTail {
		t.Error("a poll game should be created with the leaderboard suspense tail off")
	}

	// ---- import the survey-derived question set ----
	var imported struct {
		Imported  int           `json:"imported"`
		Questions []db.Question `json:"questions"`
	}
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/import",
		adminHdr, pollImportPayload, &imported)
	if imported.Imported != 2 {
		t.Fatalf("imported %d questions, want 2", imported.Imported)
	}
	// Questions are authorless: the UNIQUE (game_id, user_id) index has to
	// tolerate repeated NULLs for a host-authored set to exist at all.
	for _, q := range imported.Questions {
		if q.UserID != "" {
			t.Errorf("imported question %q has author %q, want none", q.Text, q.UserID)
		}
	}
	firstImportedText := imported.Questions[0].Text

	// ---- two teams join ----
	type joinResp struct{ Token, UserID string }
	var red, blue joinResp
	doJSON(t, "POST", httpSrv.URL+"/api/games/"+g.Code+"/join", nil, `{"name":"Team Red"}`, &red)
	doJSON(t, "POST", httpSrv.URL+"/api/games/"+g.Code+"/join", nil, `{"name":"Team Blue"}`, &blue)

	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/state", adminHdr,
		`{"state":"game"}`, http.StatusNoContent)

	// A host-authored set is uploaded in a deliberate running order, so unlike
	// classic mode it must NOT be shuffled on start.
	var ordered []db.Question
	doJSON(t, "GET", httpSrv.URL+"/api/games/"+g.Code+"/questions", nil, "", &ordered)
	if len(ordered) != 2 || ordered[0].Text != firstImportedText {
		t.Errorf("imported order not preserved: got %q first, want %q", ordered[0].Text, firstImportedText)
	}

	redWS := dialWS(t, httpSrv.URL, "?token="+red.Token)
	blueWS := dialWS(t, httpSrv.URL, "?token="+blue.Token)
	adminWS := dialWS(t, httpSrv.URL, "?role=admin&token="+login.Token+"&code="+g.Code)
	boardWS := dialWS(t, httpSrv.URL, "?role=board&code="+g.Code)

	for _, c := range []*wsClient{redWS, blueWS, adminWS, boardWS} {
		_ = c.waitFor(t, 5*time.Second, "gameState")
	}

	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/activate", adminHdr,
		`{}`, http.StatusNoContent)

	adminState := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active"
	})
	qID := stringField(adminState, "currentQuestionId")
	if nestedString(adminState, "question", "answerType") != "poll" {
		t.Fatalf("active question is not a poll question: %v", adminState)
	}

	teamState := redWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == qID
	})
	blueWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == qID
	})

	// The points ARE the answer. If a team's socket carries them before the
	// reveal, every team can read a perfect score off the network tab.
	teamOpts := pollOptionsFrom(t, teamState)
	for _, o := range teamOpts {
		if o.Points != 0 {
			t.Fatalf("point values leaked to a team before reveal: %+v", teamOpts)
		}
		if o.Text == "" {
			t.Fatalf("answer text missing from the team payload: %+v", teamOpts)
		}
	}
	// The board is a TV in the room, so it must be just as blind.
	boardState := boardWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == qID
	})
	for _, o := range pollOptionsFrom(t, boardState) {
		if o.Points != 0 {
			t.Fatalf("point values leaked to the TV board before reveal: %+v", o)
		}
	}
	// The admin needs them to run the room.
	adminOpts := pollOptionsFrom(t, adminState)
	adminTotal := 0
	for _, o := range adminOpts {
		adminTotal += o.Points
	}
	if adminTotal == 0 {
		t.Fatalf("admin should see the point values, got %+v", adminOpts)
	}

	// ---- both teams answer: Red takes the top answer, Blue the bottom one ----
	topIdx, bottomIdx := extremeIndexes(adminOpts)
	time.Sleep(80 * time.Millisecond)
	redWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": qID, "value": topIdx},
	})
	blueWS.sendJSON(t, map[string]any{
		"type": "answer",
		"data": map[string]any{"questionId": qID, "value": bottomIdx},
	})
	redWS.waitFor(t, 5*time.Second, "answerAck")
	blueWS.waitFor(t, 5*time.Second, "answerAck")

	// The board lights team names up live, so it has to receive these.
	boardWS.waitFor(t, 5*time.Second, "playerAnswered")

	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/reveal", adminHdr,
		"", http.StatusNoContent)

	revealed := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "revealed" && stringField(d, "currentQuestionId") == qID
	})
	scores := scoresByName(t, revealed)
	if scores["Team Red"] <= scores["Team Blue"] {
		t.Errorf("the top survey answer should outscore the bottom one, got %v", scores)
	}
	if scores["Team Blue"] == 0 {
		t.Errorf("every listed answer must score something, Team Blue got 0: %v", scores)
	}

	// Points are public after the reveal — the phones render the board from them.
	revealedTeam := redWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "revealed" && stringField(d, "currentQuestionId") == qID
	})
	revealedTotal := 0
	for _, o := range pollOptionsFrom(t, revealedTeam) {
		revealedTotal += o.Points
	}
	if revealedTotal == 0 {
		t.Error("teams should see the point values once the host reveals")
	}

	// With the tail off, a poll game shows the standings even on the last
	// question — the whole point of a live TV board.
	if _, hidden := revealed["leaderboardHidden"]; hidden {
		t.Error("poll game hid the leaderboard despite the suspense tail being off")
	}
}

func pollOptionsFrom(t *testing.T, state map[string]any) []game.PollOption {
	t.Helper()
	q, ok := state["question"].(map[string]any)
	if !ok {
		t.Fatalf("gameState has no question: %v", state)
	}
	raw, err := json.Marshal(q["options"])
	if err != nil {
		t.Fatalf("marshal options: %v", err)
	}
	var opts []game.PollOption
	if err := json.Unmarshal(raw, &opts); err != nil {
		t.Fatalf("unmarshal poll options from %s: %v", raw, err)
	}
	if len(opts) != 5 {
		t.Fatalf("expected 5 options, got %d (%s)", len(opts), raw)
	}
	return opts
}

// extremeIndexes returns the index of the highest- and lowest-scoring option.
func extremeIndexes(opts []game.PollOption) (top, bottom int) {
	for i, o := range opts {
		if o.Points > opts[top].Points {
			top = i
		}
		if o.Points < opts[bottom].Points {
			bottom = i
		}
	}
	if top == bottom {
		panic(fmt.Sprintf("options have no spread: %+v", opts))
	}
	return top, bottom
}

// TestIntegration_PollQuestionCRUD exercises the host's authoring flow against
// real Postgres: add several questions one at a time, edit one, reorder, and
// remove one — asserting the running order is what the host arranged, since a
// poll set is played in the order it was built.
func TestIntegration_PollQuestionCRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	d := connectDB(t, ctx, pg)

	srv := New(d, ws.NewHub(), &ai.Client{}, &mail.Mailer{})
	srv.Images = images.New(d.Pool)
	httpSrv := httptest.NewServer(srv.Routes())
	t.Cleanup(httpSrv.Close)

	var login struct{ Token string }
	doJSON(t, "POST", httpSrv.URL+"/api/admin/login", nil, `{"password":"letmein"}`, &login)
	adminHdr := http.Header{"Authorization": {"Bearer " + login.Token}}

	var g db.Game
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games", adminHdr,
		`{"name":"Authoring","questionTimeoutSeconds":90,"mode":"poll"}`, &g)

	// ---- add three questions ----
	ids := make([]string, 0, 3)
	for i := 1; i <= 3; i++ {
		var q db.Question
		body := fmt.Sprintf(`{"text":"Question %d","answers":[
			{"text":"A%d","points":40},{"text":"B%d","points":25},
			{"text":"C%d","points":15},{"text":"D%d","points":10},{"text":"E%d","points":5}]}`,
			i, i, i, i, i, i)
		doJSON(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions", adminHdr, body, &q)
		if q.ID == "" || q.AnswerType != "poll" {
			t.Fatalf("create returned %+v", q)
		}
		if q.SortOrder != i {
			t.Errorf("question %d landed at sortOrder %d, want %d", i, q.SortOrder, i)
		}
		ids = append(ids, q.ID)
	}
	assertOrder(t, httpSrv.URL, g.Code, adminHdr, []string{"Question 1", "Question 2", "Question 3"})

	// ---- edit the middle one; it must keep its slot ----
	var edited db.Question
	doJSON(t, "PUT", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/"+ids[1], adminHdr,
		`{"text":"Question 2 (edited)","answers":[
			{"text":"New A","points":50},{"text":"New B","points":20},
			{"text":"New C","points":15},{"text":"New D","points":10},{"text":"New E","points":5}]}`, &edited)
	if edited.SortOrder != 2 {
		t.Errorf("editing moved the question to slot %d, want 2", edited.SortOrder)
	}
	assertOrder(t, httpSrv.URL, g.Code, adminHdr,
		[]string{"Question 1", "Question 2 (edited)", "Question 3"})

	// ---- move the last one to the top ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/"+ids[2]+"/move",
		adminHdr, `{"direction":"up"}`, http.StatusNoContent)
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/"+ids[2]+"/move",
		adminHdr, `{"direction":"up"}`, http.StatusNoContent)
	assertOrder(t, httpSrv.URL, g.Code, adminHdr,
		[]string{"Question 3", "Question 1", "Question 2 (edited)"})

	// Moving past the top is a no-op, not an error.
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/"+ids[2]+"/move",
		adminHdr, `{"direction":"up"}`, http.StatusNoContent)
	assertOrder(t, httpSrv.URL, g.Code, adminHdr,
		[]string{"Question 3", "Question 1", "Question 2 (edited)"})

	// ---- remove one ----
	doStatus(t, "DELETE", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions/"+ids[0],
		adminHdr, "", http.StatusNoContent)
	assertOrder(t, httpSrv.URL, g.Code, adminHdr,
		[]string{"Question 3", "Question 2 (edited)"})

	// ---- the arranged order survives the start of the game ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/state", adminHdr,
		`{"state":"game"}`, http.StatusNoContent)
	assertOrder(t, httpSrv.URL, g.Code, adminHdr,
		[]string{"Question 3", "Question 2 (edited)"})

	// ---- and editing is refused once it is running ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+g.Code+"/questions", adminHdr,
		`{"text":"Too late","answers":[
			{"text":"A","points":1},{"text":"B","points":1},{"text":"C","points":1},
			{"text":"D","points":1},{"text":"E","points":1}]}`, http.StatusBadRequest)
}

func assertOrder(t *testing.T, baseURL, code string, hdr http.Header, want []string) {
	t.Helper()
	var got struct {
		Questions []db.Question `json:"questions"`
	}
	doJSON(t, "GET", baseURL+"/api/admin/games/"+code, hdr, "", &got)
	if len(got.Questions) != len(want) {
		t.Fatalf("got %d questions, want %d", len(got.Questions), len(want))
	}
	for i, q := range got.Questions {
		if q.Text != want[i] {
			texts := make([]string, len(got.Questions))
			for j, x := range got.Questions {
				texts[j] = x.Text
			}
			t.Fatalf("running order is %v, want %v", texts, want)
		}
	}
}
