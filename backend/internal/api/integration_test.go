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
	"sync"
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

// TestIntegration_TwentyPlayers exercises the system at lobby scale (20
// players) with three question types — yes/no, 4-option choice, number — and
// three edge-case player roles that don't show up in the happy-path test:
//
//   - LateJoiner: joins via HTTP AFTER the host has flipped state=game. Mid-game
//     joins are allowed (see TestJoinGameAllowedMidGame). They WS-connect
//     before the first question is activated so they can answer everything.
//   - NoQuestion: joins during setup but never authors a question. Must still
//     be able to answer and accumulate points.
//   - Ghost: joins during setup AND authors a question, then never WS-connects
//     (i.e. closes the tab before the host hits ▶). Their question must
//     survive into game mode (it's the user FK that's SET NULL, the question
//     itself stays), and they must still appear on the final leaderboard with
//     0 points (LEFT JOIN in db.Leaderboard).
//
// Scoring is set up so every correct-group player ends with the same
// per-question bonus floor — base * 4 questions = 800 — and every wrong-group
// player ends at 0, which makes the assertions deterministic against real
// Postgres timing.
func TestIntegration_TwentyPlayers(t *testing.T) {
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

	// Each question needs a distinct image (storePNGImage perturbs pixels so
	// the dedupe key differs).
	imgYesAlice := storePNGImage(t, ctx, imgSvc, 64, 48)
	imgChoiceBob := storePNGImage(t, ctx, imgSvc, 80, 60)
	imgNumberCarol := storePNGImage(t, ctx, imgSvc, 96, 72)
	imgYesGhost := storePNGImage(t, ctx, imgSvc, 112, 84)

	// ---- admin + game ----
	var login struct{ Token string }
	doJSON(t, "POST", httpSrv.URL+"/api/admin/login", nil, `{"password":"letmein"}`, &login)
	adminHdr := http.Header{"Authorization": {"Bearer " + login.Token}}

	var game db.Game
	// Bump the per-question timeout so the test isn't racing against the
	// auto-close timer if Postgres is slow on a cold start.
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games", adminHdr,
		`{"name":"Twenty","questionTimeoutSeconds":60}`, &game)

	type playerInfo struct {
		Name   string
		Token  string
		UserID string
	}
	join := func(name string) playerInfo {
		var r struct{ Token, UserID string }
		doJSON(t, "POST", httpSrv.URL+"/api/games/"+game.Code+"/join", nil,
			fmt.Sprintf(`{"name":%q}`, name), &r)
		return playerInfo{Name: name, Token: r.Token, UserID: r.UserID}
	}

	// ---- 19 players join in setup (LateJoiner stays out for now) ----
	alice := join("Alice")
	bob := join("Bob")
	carol := join("Carol")
	ghost := join("Ghost")
	noQuestion := join("NoQuestion")
	regulars := make([]playerInfo, 14)
	for i := range regulars {
		regulars[i] = join(fmt.Sprintf("Reg-%02d", i+1))
	}

	// ---- 4 questions, three of which from "real" authors plus the ghost ----
	putQ := func(token, body string) db.Question {
		var q db.Question
		doJSON(t, "PUT", httpSrv.URL+"/api/games/"+game.Code+"/questions",
			http.Header{"X-Player-Token": {token}}, body, &q)
		return q
	}
	qAlice := putQ(alice.Token, fmt.Sprintf(
		`{"text":"Sky blue?","photoImageId":%q,"answerType":"yesno","options":[],"correct":"yes"}`, imgYesAlice))
	qBob := putQ(bob.Token, fmt.Sprintf(
		`{"text":"Pick C","photoImageId":%q,"answerType":"choice","options":["A","B","C","D"],"correct":2}`, imgChoiceBob))
	qCarol := putQ(carol.Token, fmt.Sprintf(
		`{"text":"Target?","photoImageId":%q,"answerType":"number","options":[],"correct":100}`, imgNumberCarol))
	qGhost := putQ(ghost.Token, fmt.Sprintf(
		`{"text":"Reverse y/n","photoImageId":%q,"answerType":"yesno","options":[],"correct":"no"}`, imgYesGhost))

	// Sanity check the setup snapshot.
	var adminView struct {
		Questions []db.Question `json:"questions"`
		Users     []db.User     `json:"users"`
	}
	doJSON(t, "GET", httpSrv.URL+"/api/admin/games/"+game.Code, adminHdr, "", &adminView)
	if len(adminView.Users) != 19 || len(adminView.Questions) != 4 {
		t.Fatalf("setup snapshot: want 19 users + 4 questions, got %d / %d",
			len(adminView.Users), len(adminView.Questions))
	}

	// ---- transition to game; question order is randomized here ----
	doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/state", adminHdr,
		`{"state":"game"}`, http.StatusNoContent)

	// ---- LateJoiner shows up after kickoff ----
	lateJoiner := join("LateJoiner")

	// Ghost's question must survive the state transition even though Ghost
	// never connected: the questions table's user FK is SET NULL on delete,
	// so the question row itself stays.
	doJSON(t, "GET", httpSrv.URL+"/api/admin/games/"+game.Code, adminHdr, "", &adminView)
	if len(adminView.Users) != 20 || len(adminView.Questions) != 4 {
		t.Fatalf("post-game snapshot: want 20 users + 4 questions, got %d / %d",
			len(adminView.Users), len(adminView.Questions))
	}

	// Map questionID → answer plan so the test can pick the right value once
	// the random order is observed at activation time.
	type qPlan struct {
		AnswerType string
		Correct    any
		Wrong      any
	}
	plans := map[string]qPlan{
		qAlice.ID: {"yesno", "yes", "no"},
		qBob.ID:   {"choice", 2, 0},
		qCarol.ID: {"number", 100, 1000},
		qGhost.ID: {"yesno", "no", "yes"},
	}

	// ---- WS connect for everyone except Ghost ----
	type playerWS struct {
		info playerInfo
		ws   *wsClient
	}
	playWS := []playerWS{
		{alice, dialWS(t, httpSrv.URL, "?token="+alice.Token)},
		{bob, dialWS(t, httpSrv.URL, "?token="+bob.Token)},
		{carol, dialWS(t, httpSrv.URL, "?token="+carol.Token)},
		{noQuestion, dialWS(t, httpSrv.URL, "?token="+noQuestion.Token)},
		{lateJoiner, dialWS(t, httpSrv.URL, "?token="+lateJoiner.Token)},
	}
	for _, r := range regulars {
		playWS = append(playWS, playerWS{r, dialWS(t, httpSrv.URL, "?token="+r.Token)})
	}
	adminWS := dialWS(t, httpSrv.URL, "?role=admin&token="+login.Token+"&code="+game.Code)

	// Drain the initial gameState each client gets on join.
	for _, p := range playWS {
		_ = p.ws.waitFor(t, 5*time.Second, "gameState")
	}
	_ = adminWS.waitFor(t, 5*time.Second, "gameState")

	// Correct group: 15 players (Alice, Bob, Carol, NoQuestion, LateJoiner +
	// 10 regulars). Wrong group: the last 4 regulars. Ghost: not in either,
	// will appear on the leaderboard with 0 points via the LEFT JOIN.
	correct := map[string]bool{
		alice.Name: true, bob.Name: true, carol.Name: true,
		noQuestion.Name: true, lateJoiner.Name: true,
	}
	for i := 0; i < 10; i++ {
		correct[regulars[i].Name] = true
	}
	wrongRegulars := regulars[10:]

	// ---- run all four questions ----
	for i := 0; i < 4; i++ {
		qid := advanceToNext(t, httpSrv.URL, game.Code, adminHdr, adminWS, i == 0)
		plan, ok := plans[qid]
		if !ok {
			t.Fatalf("activated unknown question id %q", qid)
		}

		// Wait for every player WS to see the new active question before any
		// of them answer; otherwise a too-early answer races the broadcast.
		for _, p := range playWS {
			p.ws.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
				return d["questionState"] == "active" && stringField(d, "currentQuestionId") == qid
			})
		}

		// Give the question a measurable lifetime so responseMs > 0 and the
		// time bonus exercises its decay branch (otherwise it'd be the
		// degenerate base + base/2 every time).
		time.Sleep(80 * time.Millisecond)

		// Answer in parallel: 19 client sockets running through the hub at
		// once is the realistic case and roughly matches the capacity
		// reasoning in the README ("80 simultaneous answers → 240 queries").
		var wg sync.WaitGroup
		for _, p := range playWS {
			p := p
			wg.Add(1)
			go func() {
				defer wg.Done()
				value := plan.Wrong
				if correct[p.info.Name] {
					value = plan.Correct
				}
				p.ws.sendJSON(t, map[string]any{
					"type": "answer",
					"data": map[string]any{"questionId": qid, "value": value},
				})
			}()
		}
		wg.Wait()

		// Every connected player should get their personal answerAck back.
		for _, p := range playWS {
			p.ws.waitFor(t, 10*time.Second, "answerAck")
		}

		doStatus(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/reveal", adminHdr,
			"", http.StatusNoContent)
		_ = adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
			return d["questionState"] == "revealed" && stringField(d, "currentQuestionId") == qid
		})
	}

	// One more "next" closes the game.
	var nextResp struct {
		Done       bool
		QuestionID string
	}
	doJSON(t, "POST", httpSrv.URL+"/api/admin/games/"+game.Code+"/next", adminHdr, "", &nextResp)
	if !nextResp.Done {
		t.Fatalf("expected done=true after 4 questions, got %+v", nextResp)
	}

	// ---- final assertions on the real DB leaderboard ----
	var leaderboard []db.Score
	doJSON(t, "GET", httpSrv.URL+"/api/games/"+game.Code+"/leaderboard", nil, "", &leaderboard)
	if len(leaderboard) != 20 {
		t.Fatalf("expected 20 leaderboard rows (incl. Ghost), got %d", len(leaderboard))
	}

	score := map[string]db.Score{}
	for _, s := range leaderboard {
		score[s.UserName] = s
	}

	// Correct group: base sum is 100+300+300+100 = 800. Time bonus is
	// strictly < base/2 per question (it'd be == base/2 only at responseMs=0,
	// which never happens), so the ceiling is 100·1.5 + 300·1.5 + 300·1.5 +
	// 100·1.5 = 1200. Allow a small headroom for any rounding quirks.
	for name := range correct {
		s, ok := score[name]
		if !ok {
			t.Errorf("missing leaderboard row for %s", name)
			continue
		}
		if s.Points < 800 || s.Points > 1200 {
			t.Errorf("%s: expected 800 ≤ points ≤ 1200, got %d", name, s.Points)
		}
		if s.Correct != 4 {
			t.Errorf("%s: expected 4 correct, got %d", name, s.Correct)
		}
	}

	// Wrong group: deliberately wrong on every question, including the number
	// (1000 vs. correct=100 puts them out of the rank-3 closeness band).
	for _, w := range wrongRegulars {
		s, ok := score[w.Name]
		if !ok {
			t.Errorf("missing leaderboard row for wrong player %s", w.Name)
			continue
		}
		if s.Points != 0 || s.Correct != 0 {
			t.Errorf("%s: expected 0/0, got %d/%d", w.Name, s.Points, s.Correct)
		}
	}

	// Ghost authored a question and never WS-connected, so they submitted
	// nothing — the LEFT JOIN in db.Leaderboard should still surface them.
	if g, ok := score[ghost.Name]; !ok {
		t.Errorf("Ghost missing from leaderboard")
	} else if g.Points != 0 || g.Correct != 0 {
		t.Errorf("Ghost: expected 0/0 (never answered), got %d/%d", g.Points, g.Correct)
	}

	// LateJoiner answered every question — must hit the same correct-group
	// floor, which confirms a mid-game join can still score on Q1 (the one
	// they technically missed the setup for).
	if s := score[lateJoiner.Name]; s.Correct != 4 {
		t.Errorf("LateJoiner: expected 4 correct, got %d (points=%d)", s.Correct, s.Points)
	}

	// Game should be finished.
	var final struct {
		State string `json:"state"`
	}
	doJSON(t, "GET", httpSrv.URL+"/api/games/"+game.Code, nil, "", &final)
	if final.State != "finished" {
		t.Errorf("game state: want finished, got %q", final.State)
	}
}

// advanceToNext activates the first question (when first==true) or asks the
// admin to advance to the next one, then waits for the admin's WS to see the
// active state and returns the new currentQuestionId.
func advanceToNext(t *testing.T, baseURL, code string, adminHdr http.Header, adminWS *wsClient, first bool) string {
	t.Helper()
	if first {
		doStatus(t, "POST", baseURL+"/api/admin/games/"+code+"/activate", adminHdr,
			`{}`, http.StatusNoContent)
		state := adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
			return d["questionState"] == "active"
		})
		return stringField(state, "currentQuestionId")
	}
	var nextResp struct {
		Done       bool
		QuestionID string
	}
	doJSON(t, "POST", baseURL+"/api/admin/games/"+code+"/next", adminHdr, "", &nextResp)
	if nextResp.Done {
		t.Fatalf("/next returned done=true before all questions were exhausted")
	}
	_ = adminWS.waitForGameStateWhere(t, 5*time.Second, func(d map[string]any) bool {
		return d["questionState"] == "active" && stringField(d, "currentQuestionId") == nextResp.QuestionID
	})
	return nextResp.QuestionID
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
