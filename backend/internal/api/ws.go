package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/oglimmer/trivia/backend/internal/auth"
	"github.com/oglimmer/trivia/backend/internal/game"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

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
	if c.Role == ws.RolePlayer && c.UserID != "" {
		if err := s.DB.TouchUserLastSeen(ctx, c.UserID); err != nil {
			log.Printf("touch last_seen on ws join for %s: %v", c.UserID, err)
		}
	}
	g, err := s.DB.GameByID(ctx, c.GameID)
	if err != nil {
		return
	}
	// If a player has already answered the currently-active question, replay an
	// answerAck before gameState so a page reload mid-question lands on the
	// "Locked in!" view instead of the answer buttons.
	if c.Role == ws.RolePlayer && g.QuestionState == "active" && g.CurrentQuestionID != nil {
		if ans, err := s.DB.AnswersForQuestion(ctx, *g.CurrentQuestionID); err == nil {
			for _, a := range ans {
				if a.UserID == c.UserID {
					c.Send(map[string]any{
						"type": "answerAck",
						"data": map[string]any{"questionId": a.QuestionID, "responseMs": a.ResponseMs},
					})
					break
				}
			}
		}
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
		if c.UserID != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.DB.TouchUserLastSeen(ctx, c.UserID); err != nil {
				log.Printf("touch last_seen on ws leave for %s: %v", c.UserID, err)
			}
		}
		s.broadcastPresence(c.GameID)
	}
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
	// Reject answers that race in after the timeout — protects against a small
	// window where the auto-close timer has not yet flipped question_state.
	if g.QuestionTimeoutSeconds > 0 && responseMs > g.QuestionTimeoutSeconds*1000 {
		return
	}
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
