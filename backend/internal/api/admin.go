package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/auth"
)

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
	Code                   string     `json:"code"`
	Name                   string     `json:"name"`
	QuestionTimeoutSeconds int        `json:"questionTimeoutSeconds"`
	ScheduledAt            *time.Time `json:"scheduledAt"`
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
			"id":                     g.ID,
			"code":                   g.Code,
			"name":                   g.Name,
			"state":                  g.State,
			"currentQuestionId":      g.CurrentQuestionID,
			"questionState":          g.QuestionState,
			"questionStartedAt":      g.QuestionStartedAt,
			"questionClosedAt":       g.QuestionClosedAt,
			"questionTimeoutSeconds": g.QuestionTimeoutSeconds,
			"scheduledAt":            g.ScheduledAt,
			"createdAt":              g.CreatedAt,
			"onlineCount":            counts[g.ID],
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) listAllUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.DB.ListAllUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, us)
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
	g, err := s.DB.CreateGame(r.Context(), b.Code, b.Name, clampTimeout(b.QuestionTimeoutSeconds), b.ScheduledAt)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, 200, g)
}

func (s *Server) adminGame(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
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
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	s.Hub.Broadcast(g.ID, map[string]any{"type": "gameDeleted"})
	s.cancelAutoClose(g.ID)
	if err := s.DB.DeleteGame(r.Context(), g.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.dropGameLock(g.ID)
	// The game's users/questions are gone via ON DELETE CASCADE, leaving any
	// images they pointed at unreferenced. Sweep them now instead of waiting
	// for the periodic GC tick.
	s.deleteOrphanImages(r.Context(), time.Now().Add(-orphanImageGrace))
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "players can only be removed in setup")
		return
	}
	userID := chi.URLParam(r, "userId")
	u, err := s.DB.UserByID(r.Context(), userID)
	if err != nil || u.GameID != g.ID {
		writeErr(w, http.StatusNotFound, "no user")
		return
	}
	if err := s.DB.DeleteUser(r.Context(), userID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastUsers(r.Context(), g.ID)
	s.broadcastQuestionsAdmin(r.Context(), g.ID)
	s.broadcastPresence(g.ID)
	w.WriteHeader(204)
}

// impersonateUser returns the player's token so an admin can construct a
// login link that signs them in as that player.
func (s *Server) impersonateUser(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	userID := chi.URLParam(r, "userId")
	u, err := s.DB.UserByID(r.Context(), userID)
	if err != nil || u.GameID != g.ID {
		writeErr(w, http.StatusNotFound, "no user")
		return
	}
	tok, err := s.DB.UserTokenByID(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"token":  tok,
		"userId": u.ID,
		"code":   g.Code,
	})
}

func (s *Server) deleteQuestion(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "questions can only be removed in setup")
		return
	}
	qID := chi.URLParam(r, "questionId")
	q, err := s.DB.QuestionByID(r.Context(), qID)
	if err != nil || q.GameID != g.ID {
		writeErr(w, http.StatusNotFound, "no question")
		return
	}
	if err := s.DB.DeleteQuestion(r.Context(), qID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastQuestionsAdmin(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) setGameState(w http.ResponseWriter, r *http.Request) {
	var b struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if b.State != "setup" && b.State != "game" && b.State != "finished" {
		writeErr(w, http.StatusBadRequest, "bad state")
		return
	}
	prunedUsers := false
	if b.State == "game" {
		// Drop players who haven't been seen in the last 30 minutes — they've
		// most likely abandoned the lobby and shouldn't appear on the
		// leaderboard. Their questions stay (the FK is ON DELETE SET NULL).
		cutoff := time.Now().Add(-staleUserThreshold)
		removed, err := s.DB.DeleteStaleUsers(r.Context(), g.ID, cutoff)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		prunedUsers = len(removed) > 0
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
	// No active question survives a state transition.
	s.cancelAutoClose(g.ID)
	if prunedUsers {
		s.broadcastUsers(r.Context(), g.ID)
		s.broadcastQuestionsAdmin(r.Context(), g.ID)
		s.broadcastPresence(g.ID)
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) updateGameSettings(w http.ResponseWriter, r *http.Request) {
	// json.RawMessage lets us tell "field absent" from "explicit null" — clearing
	// the scheduled date is a real action and must be representable.
	var b struct {
		QuestionTimeoutSeconds *int            `json:"questionTimeoutSeconds"`
		ScheduledAt            json.RawMessage `json:"scheduledAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	// Lock the value once players are in the game, so a mid-game change can't
	// mismatch the timer that is already running.
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "settings can only be changed in setup")
		return
	}
	if b.QuestionTimeoutSeconds != nil {
		if err := s.DB.SetQuestionTimeout(r.Context(), g.ID, clampTimeout(*b.QuestionTimeoutSeconds)); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(b.ScheduledAt) > 0 {
		var sched *time.Time
		raw := strings.TrimSpace(string(b.ScheduledAt))
		if raw != "null" {
			var t time.Time
			if err := json.Unmarshal(b.ScheduledAt, &t); err != nil {
				writeErr(w, http.StatusBadRequest, "scheduledAt must be RFC3339 timestamp or null")
				return
			}
			sched = &t
		}
		if err := s.DB.SetGameScheduledAt(r.Context(), g.ID, sched); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) activateQuestion(w http.ResponseWriter, r *http.Request) {
	var b struct {
		QuestionID string `json:"questionId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&b)
	g := s.loadGameByCode(w, r)
	if g == nil {
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
	if s.Metrics != nil {
		s.Metrics.QuestionsActivated.Inc()
	}
	if g.QuestionTimeoutSeconds > 0 {
		s.scheduleAutoClose(g.ID, qID, time.Duration(g.QuestionTimeoutSeconds)*time.Second)
	} else {
		s.cancelAutoClose(g.ID)
	}
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) revealQuestion(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
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
	if s.Metrics != nil {
		s.Metrics.QuestionsRevealed.Inc()
	}
	s.cancelAutoClose(g.ID)
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}

func (s *Server) nextQuestion(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	qs, _ := s.DB.ListQuestions(r.Context(), g.ID, false)
	next := pickNext(qs, g.CurrentQuestionID)
	if next == nil {
		if err := s.DB.SetGameState(r.Context(), g.ID, "finished"); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.cancelAutoClose(g.ID)
		s.broadcastGameState(r.Context(), g.ID)
		writeJSON(w, 200, map[string]any{"done": true})
		return
	}
	if err := s.DB.ActivateQuestion(r.Context(), g.ID, next.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.Metrics != nil {
		s.Metrics.QuestionsActivated.Inc()
	}
	if g.QuestionTimeoutSeconds > 0 {
		s.scheduleAutoClose(g.ID, next.ID, time.Duration(g.QuestionTimeoutSeconds)*time.Second)
	} else {
		s.cancelAutoClose(g.ID)
	}
	s.broadcastGameState(r.Context(), g.ID)
	writeJSON(w, 200, map[string]any{"done": false, "questionId": next.ID})
}

func (s *Server) finishGame(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if err := s.DB.SetGameState(r.Context(), g.ID, "finished"); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cancelAutoClose(g.ID)
	s.broadcastGameState(r.Context(), g.ID)
	w.WriteHeader(204)
}
