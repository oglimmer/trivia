package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/auth"
	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/game"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// Server is the HTTP API plus the live WebSocket hub.
type Server struct {
	DB  *db.DB
	Hub *ws.Hub
	AI  *ai.Client

	// gameLocks serializes admin transitions per game.
	mu        sync.Mutex
	gameLocks map[string]*sync.Mutex
}

func New(d *db.DB, h *ws.Hub, c *ai.Client) *Server {
	s := &Server{DB: d, Hub: h, AI: c, gameLocks: map[string]*sync.Mutex{}}
	h.OnRecv = s.onWSMessage
	h.OnJoin = s.onWSJoin
	h.OnLeave = s.onWSLeave
	return s
}

func (s *Server) lockFor(gameID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.gameLocks[gameID]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.gameLocks[gameID] = m
	return m
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Post("/admin/login", s.adminLogin)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/games", s.listGames)
			r.Post("/admin/games", s.createGame)
			r.Get("/admin/games/{code}", s.adminGame)
			r.Delete("/admin/games/{code}", s.deleteGame)
			r.Post("/admin/games/{code}/state", s.setGameState)
			r.Post("/admin/games/{code}/activate", s.activateQuestion)
			r.Post("/admin/games/{code}/reveal", s.revealQuestion)
			r.Post("/admin/games/{code}/next", s.nextQuestion)
			r.Post("/admin/games/{code}/finish", s.finishGame)
		})

		// Player-facing endpoints.
		r.Get("/games/{code}", s.getGameForJoin)
		r.Post("/games/{code}/join", s.joinGame)
		r.Get("/me", s.me)
		r.Put("/me", s.updateMe)
		r.Get("/games/{code}/users", s.listUsersPublic)
		r.Get("/games/{code}/questions", s.listQuestionsPublic)
		r.Put("/games/{code}/questions", s.putQuestion)
		r.Get("/games/{code}/leaderboard", s.leaderboard)
		r.Post("/ai/suggest", s.aiSuggest)
	})

	// WebSocket entry point.
	r.Get("/ws", s.serveWS)

	return r
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// playerFromHeader looks up a player by their X-Player-Token header.
func (s *Server) playerFromHeader(r *http.Request) (*db.User, error) {
	tok := r.Header.Get("X-Player-Token")
	if tok == "" {
		return nil, errors.New("missing player token")
	}
	return s.DB.UserByToken(r.Context(), tok)
}

// ---------- admin login ----------

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	expected := os.Getenv("ADMIN_PASSWORD")
	if expected == "" {
		expected = "letmein"
	}
	if subtle.ConstantTimeCompare([]byte(body.Password), []byte(expected)) != 1 {
		writeErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	tok, err := auth.Issue("admin", "admin", 24*time.Hour)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"token": tok})
}

// ---------- admin games ----------

type createGameBody struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	gs, err := s.DB.ListGames(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	counts := s.Hub.OnlinePlayerCounts()
	out := make([]map[string]any, 0, len(gs))
	for _, g := range gs {
		out = append(out, map[string]any{
			"id":                g.ID,
			"code":              g.Code,
			"name":              g.Name,
			"state":             g.State,
			"currentQuestionId": g.CurrentQuestionID,
			"questionState":     g.QuestionState,
			"questionStartedAt": g.QuestionStartedAt,
			"questionClosedAt":  g.QuestionClosedAt,
			"createdAt":         g.CreatedAt,
			"onlineCount":       counts[g.ID],
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var b createGameBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	b.Code = strings.ToLower(strings.TrimSpace(b.Code))
	if b.Code == "" {
		b.Code = randomCode()
	}
	g, err := s.DB.CreateGame(r.Context(), b.Code, b.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, 200, g)
}

func (s *Server) adminGame(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	users, _ := s.DB.ListUsers(r.Context(), g.ID)
	qs, _ := s.DB.ListQuestions(r.Context(), g.ID, true)
	writeJSON(w, 200, map[string]any{
		"game":      g,
		"users":     users,
		"questions": qs,
		"online":    s.Hub.OnlinePlayers(g.ID),
	})
}

func (s *Server) deleteGame(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	s.Hub.Broadcast(g.ID, map[string]any{"type": "gameDeleted"})
	if err := s.DB.DeleteGame(r.Context(), g.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) setGameState(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var b struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if b.State != "setup" && b.State != "game" && b.State != "finished" {
		writeErr(w, http.StatusBadRequest, "bad state")
		return
	}
	if b.State == "game" {
		// Shuffle question order before entering game mode.
		if err := s.DB.RandomizeQuestionOrder(r.Context(), g.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.DB.ClearCurrentQuestion(r.Context(), g.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := s.DB.SetGameState(r.Context(), g.ID, b.State); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) activateQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var b struct {
		QuestionID string `json:"questionId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&b)
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if g.State != "game" {
		writeErr(w, http.StatusBadRequest, "game not in 'game' state")
		return
	}
	lock := s.lockFor(g.ID)
	lock.Lock()
	defer lock.Unlock()

	// If no question id provided, pick the next in sort order.
	qID := b.QuestionID
	if qID == "" {
		qs, _ := s.DB.ListQuestions(r.Context(), g.ID, false)
		next := pickNext(qs, g.CurrentQuestionID)
		if next == nil {
			writeErr(w, http.StatusBadRequest, "no more questions")
			return
		}
		qID = next.ID
	}
	if err := s.DB.ActivateQuestion(r.Context(), g.ID, qID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) revealQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if g.QuestionState != "active" {
		writeErr(w, http.StatusBadRequest, "no active question")
		return
	}
	if g.CurrentQuestionID != nil {
		if err := s.rescoreNumberAnswers(r.Context(), *g.CurrentQuestionID); err != nil {
			log.Printf("rescore number answers: %v", err)
		}
	}
	if err := s.DB.CloseQuestion(r.Context(), g.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

// rescoreNumberAnswers ranks all submitted answers to a number question by
// closeness and writes the resulting points back to the answers table. This is
// a no-op for non-number questions.
func (s *Server) rescoreNumberAnswers(ctx context.Context, questionID string) error {
	q, err := s.DB.QuestionByID(ctx, questionID)
	if err != nil {
		return err
	}
	if q.AnswerType != "number" {
		return nil
	}
	ans, err := s.DB.AnswersForQuestion(ctx, questionID)
	if err != nil {
		return err
	}
	if len(ans) == 0 {
		return nil
	}
	inputs := make([]game.NumberAnswer, len(ans))
	for i, a := range ans {
		inputs[i] = game.NumberAnswer{UserID: a.UserID, Answer: a.Answer, ResponseMs: a.ResponseMs}
	}
	scores := game.ScoreNumberAnswers(q.Correct, inputs)
	for _, sc := range scores {
		if err := s.DB.UpdateAnswerScore(ctx, questionID, sc.UserID, sc.IsCorrect, sc.Points); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) nextQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	qs, _ := s.DB.ListQuestions(r.Context(), g.ID, false)
	next := pickNext(qs, g.CurrentQuestionID)
	if next == nil {
		if err := s.DB.SetGameState(r.Context(), g.ID, "finished"); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.broadcastGameState(r.Context(), g.ID)
		writeJSON(w, 200, map[string]any{"done": true})
		return
	}
	if err := s.DB.ActivateQuestion(r.Context(), g.ID, next.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastGameState(r.Context(), g.ID)
	writeJSON(w, 200, map[string]any{"done": false, "questionId": next.ID})
}

func (s *Server) finishGame(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if err := s.DB.SetGameState(r.Context(), g.ID, "finished"); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

// pickNext returns the next question after currentID, or nil if at end.
// Caller passes questions ordered by sort_order.
func pickNext(qs []db.Question, currentID *string) *db.Question {
	if currentID == nil {
		if len(qs) == 0 {
			return nil
		}
		q := qs[0]
		return &q
	}
	for i, q := range qs {
		if q.ID == *currentID {
			if i+1 < len(qs) {
				next := qs[i+1]
				return &next
			}
			return nil
		}
	}
	if len(qs) > 0 {
		q := qs[0]
		return &q
	}
	return nil
}

// ---------- player endpoints ----------

func (s *Server) getGameForJoin(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	writeJSON(w, 200, map[string]any{
		"code":  g.Code,
		"name":  g.Name,
		"state": g.State,
	})
}

func (s *Server) joinGame(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var b struct {
		Name     string `json:"name"`
		PhotoB64 string `json:"photoB64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "game not in setup")
		return
	}
	u, err := s.DB.CreateUser(r.Context(), g.ID, b.Name, b.PhotoB64, randomToken(16))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastUsers(r.Context(), g.ID)
	writeJSON(w, 200, map[string]any{
		"token":  u.Token,
		"userId": u.ID,
		"gameId": g.ID,
		"code":   g.Code,
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	g, _ := s.DB.GameByID(r.Context(), u.GameID)
	writeJSON(w, 200, map[string]any{
		"user": u,
		"game": g,
	})
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	var b struct {
		Name     string `json:"name"`
		PhotoB64 string `json:"photoB64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	if strings.TrimSpace(b.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.DB.UpdateUser(r.Context(), u.ID, b.Name, b.PhotoB64); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastUsers(r.Context(), u.GameID)
	w.WriteHeader(204)
}

func (s *Server) listUsersPublic(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	users, err := s.DB.ListUsers(r.Context(), g.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, users)
}

func (s *Server) listQuestionsPublic(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	// Only return correct answers if game is finished. Otherwise strip.
	includeCorrect := g.State == "finished"
	qs, err := s.DB.ListQuestions(r.Context(), g.ID, includeCorrect)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, qs)
}

type putQuestionBody struct {
	Text       string          `json:"text"`
	PhotoB64   string          `json:"photoB64"`
	AnswerType string          `json:"answerType"`
	Options    json.RawMessage `json:"options"`
	Correct    json.RawMessage `json:"correct"`
}

func (s *Server) putQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	if g.ID != u.GameID {
		writeErr(w, http.StatusForbidden, "wrong game")
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "not in setup")
		return
	}
	var b putQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	if err := validateQuestion(b); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(b.Options) == 0 {
		b.Options = json.RawMessage("[]")
	}
	q, err := s.DB.UpsertQuestion(r.Context(), g.ID, u.ID, b.Text, b.PhotoB64, b.AnswerType, b.Options, b.Correct)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastQuestionsAdmin(r.Context(), g.ID)
	writeJSON(w, 200, q)
}

func validateQuestion(b putQuestionBody) error {
	if strings.TrimSpace(b.Text) == "" {
		return errors.New("text required")
	}
	if b.PhotoB64 == "" {
		return errors.New("photo required")
	}
	switch b.AnswerType {
	case "yesno":
		var v string
		if err := json.Unmarshal(b.Correct, &v); err != nil {
			return errors.New("correct must be 'yes' or 'no'")
		}
		if v != "yes" && v != "no" {
			return errors.New("correct must be 'yes' or 'no'")
		}
	case "choice":
		var opts []string
		if err := json.Unmarshal(b.Options, &opts); err != nil || len(opts) < 2 || len(opts) > 4 {
			return errors.New("options must be 2-4 strings")
		}
		var idx int
		if err := json.Unmarshal(b.Correct, &idx); err != nil || idx < 0 || idx >= len(opts) {
			return errors.New("correct must be a valid option index")
		}
	case "number":
		var n float64
		if err := json.Unmarshal(b.Correct, &n); err != nil {
			return errors.New("correct must be a number")
		}
	default:
		return errors.New("answerType must be yesno|choice|number")
	}
	return nil
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no game")
		return
	}
	sc, err := s.DB.Leaderboard(r.Context(), g.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, sc)
}

// ---------- AI ----------

func (s *Server) aiSuggest(w http.ResponseWriter, r *http.Request) {
	var req ai.SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	res, err := s.AI.Suggest(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

// ---------- WebSocket ----------

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	role := ws.RolePlayer
	gameID := ""
	userID := ""

	// Admin path: ?role=admin&token=<jwt>&code=<code>
	if r.URL.Query().Get("role") == "admin" {
		tok := r.URL.Query().Get("token")
		c, err := auth.Parse(tok)
		if err != nil || c.Role != "admin" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		code := r.URL.Query().Get("code")
		g, err := s.DB.GameByCode(r.Context(), code)
		if err != nil {
			http.Error(w, "no game", http.StatusNotFound)
			return
		}
		role = ws.RoleAdmin
		gameID = g.ID
	} else {
		tok := r.URL.Query().Get("token")
		u, err := s.DB.UserByToken(r.Context(), tok)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		gameID = u.GameID
		userID = u.ID
	}

	s.Hub.Serve(w, r, gameID, userID, role)
}

func (s *Server) onWSJoin(c *ws.Client) {
	// On join (initial connect or a wake-time reconnect), push enough state for
	// the client to fully refresh without an extra HTTP round-trip.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	g, err := s.DB.GameByID(ctx, c.GameID)
	if err != nil {
		return
	}
	c.Send(s.gameStateEnvelope(ctx, g, c.Role == ws.RoleAdmin))
	if users, err := s.DB.ListUsers(ctx, c.GameID); err == nil {
		c.Send(map[string]any{"type": "users", "data": users})
	}
	if c.Role == ws.RoleAdmin {
		if qs, err := s.DB.ListQuestions(ctx, c.GameID, true); err == nil {
			c.Send(map[string]any{"type": "questionsAdmin", "data": qs})
		}
		c.Send(s.presenceEnvelope(c.GameID))
	} else if c.Role == ws.RolePlayer {
		s.broadcastPresence(c.GameID)
	}
}

func (s *Server) onWSLeave(c *ws.Client) {
	if c.Role == ws.RolePlayer {
		s.broadcastPresence(c.GameID)
	}
}

func (s *Server) presenceEnvelope(gameID string) map[string]any {
	return map[string]any{
		"type": "presence",
		"data": map[string]any{"online": s.Hub.OnlinePlayers(gameID)},
	}
}

func (s *Server) broadcastPresence(gameID string) {
	msg := s.presenceEnvelope(gameID)
	s.Hub.BroadcastTo(gameID, msg, func(c *ws.Client) bool { return c.Role == ws.RoleAdmin })
}

type wsInbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (s *Server) onWSMessage(c *ws.Client, b []byte) {
	var m wsInbound
	if err := json.Unmarshal(b, &m); err != nil {
		return
	}
	switch m.Type {
	case "answer":
		s.handleAnswer(c, m.Data)
	case "ping":
		c.Send(map[string]any{"type": "pong"})
	}
}

type answerMsg struct {
	QuestionID string          `json:"questionId"`
	Value      json.RawMessage `json:"value"`
}

func (s *Server) handleAnswer(c *ws.Client, data json.RawMessage) {
	if c.Role != ws.RolePlayer {
		return
	}
	var m answerMsg
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	g, err := s.DB.GameByID(ctx, c.GameID)
	if err != nil {
		return
	}
	if g.QuestionState != "active" || g.CurrentQuestionID == nil || *g.CurrentQuestionID != m.QuestionID {
		return
	}
	q, err := s.DB.QuestionByID(ctx, m.QuestionID)
	if err != nil {
		return
	}
	// Compute response time from question_started_at.
	if g.QuestionStartedAt == nil {
		return
	}
	responseMs := int(time.Since(*g.QuestionStartedAt) / time.Millisecond)
	optCount := game.OptionCount(q.AnswerType, q.Options)
	ok, pts := game.JudgeAnswer(q.AnswerType, optCount, q.Correct, m.Value, responseMs)
	if err := s.DB.SaveAnswer(ctx, q.ID, c.UserID, m.Value, responseMs, ok, pts); err != nil {
		log.Printf("save answer: %v", err)
	}
	// Echo personal ack to player; broadcast generic "someone answered" to admin only.
	c.Send(map[string]any{
		"type": "answerAck",
		"data": map[string]any{"questionId": q.ID, "responseMs": responseMs},
	})
	s.Hub.BroadcastTo(c.GameID, map[string]any{
		"type": "playerAnswered",
		"data": map[string]any{"userId": c.UserID, "questionId": q.ID, "responseMs": responseMs},
	}, func(cl *ws.Client) bool { return cl.Role == ws.RoleAdmin })
}

// ---------- broadcasts ----------

// gameStateEnvelope builds the payload describing the current view of a game.
// For admin, the current question includes correct/options; for players, correct is stripped during active.
func (s *Server) gameStateEnvelope(ctx context.Context, g *db.Game, asAdmin bool) map[string]any {
	out := map[string]any{
		"type": "gameState",
		"data": map[string]any{
			"code":              g.Code,
			"name":              g.Name,
			"state":             g.State,
			"questionState":     g.QuestionState,
			"currentQuestionId": g.CurrentQuestionID,
			"questionStartedAt": g.QuestionStartedAt,
		},
	}
	data := out["data"].(map[string]any)

	if g.CurrentQuestionID != nil {
		q, err := s.DB.QuestionByID(ctx, *g.CurrentQuestionID)
		if err == nil {
			qd := map[string]any{
				"id":         q.ID,
				"text":       q.Text,
				"photoB64":   q.PhotoB64,
				"answerType": q.AnswerType,
				"options":    q.Options,
				"userId":     q.UserID,
			}
			if asAdmin || g.QuestionState == "revealed" {
				qd["correct"] = q.Correct
			}
			data["question"] = qd
			if g.QuestionState == "revealed" {
				ans, _ := s.DB.AnswersForQuestion(ctx, q.ID)
				data["answers"] = ans
			}
		}
	}

	if g.State == "finished" || g.QuestionState == "revealed" {
		lb, _ := s.DB.Leaderboard(ctx, g.ID)
		data["leaderboard"] = lb
	}

	return out
}

func (s *Server) broadcastGameState(ctx context.Context, gameID string) {
	g, err := s.DB.GameByID(ctx, gameID)
	if err != nil {
		return
	}
	playerMsg := s.gameStateEnvelope(ctx, g, false)
	adminMsg := s.gameStateEnvelope(ctx, g, true)
	s.Hub.BroadcastTo(gameID, playerMsg, func(c *ws.Client) bool { return c.Role == ws.RolePlayer })
	s.Hub.BroadcastTo(gameID, adminMsg, func(c *ws.Client) bool { return c.Role == ws.RoleAdmin })
}

func (s *Server) broadcastUsers(ctx context.Context, gameID string) {
	users, err := s.DB.ListUsers(ctx, gameID)
	if err != nil {
		return
	}
	s.Hub.Broadcast(gameID, map[string]any{"type": "users", "data": users})
}

func (s *Server) broadcastQuestionsAdmin(ctx context.Context, gameID string) {
	qs, err := s.DB.ListQuestions(ctx, gameID, true)
	if err != nil {
		return
	}
	s.Hub.BroadcastTo(gameID, map[string]any{"type": "questionsAdmin", "data": qs},
		func(c *ws.Client) bool { return c.Role == ws.RoleAdmin })
}

// randomCode generates a short alphanumeric game code.
func randomCode() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 1)
	out := make([]byte, 4)
	for i := range out {
		_, _ = rand.Read(b)
		out[i] = alphabet[int(b[0])%len(alphabet)]
	}
	return string(out)
}
