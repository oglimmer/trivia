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

	// Board path: ?role=board&code=<code>. No token — a TV in the room is not a
	// participant, so it gets no player identity and can only listen.
	if r.URL.Query().Get("role") == "board" {
		g, err := s.DB.GameByCode(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "no game", http.StatusNotFound)
			return
		}
		start := time.Now()
		s.Hub.Serve(w, r, g.ID, "", ws.RoleBoard)
		if s.Metrics != nil {
			s.Metrics.RecordWSSession(string(ws.RoleBoard), time.Since(start))
		}
		return
	}

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

	start := time.Now()
	s.Hub.Serve(w, r, gameID, userID, role)
	if s.Metrics != nil {
		s.Metrics.RecordWSSession(string(role), time.Since(start))
	}
}

func (s *Server) onWSJoin(c *ws.Client) {
	if s.Metrics != nil {
		s.Metrics.WSConnect(string(c.Role))
	}
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
		log.Printf("ws join game lookup for %s: %v", c.GameID, err)
		return
	}
	// If a player has already answered the currently-active question, replay an
	// answerAck before gameState so a page reload mid-question lands on the
	// "Locked in!" view instead of the answer buttons.
	if c.Role == ws.RolePlayer && g.QuestionState == "active" && g.CurrentQuestionID != nil {
		ans, err := s.DB.AnswersForQuestion(ctx, *g.CurrentQuestionID)
		if err != nil {
			log.Printf("ws join answers for question %s: %v", *g.CurrentQuestionID, err)
		} else {
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
	users, err := s.DB.ListUsers(ctx, c.GameID)
	if err != nil {
		log.Printf("ws join list users for %s: %v", c.GameID, err)
	} else {
		c.Send(map[string]any{"type": "users", "data": users})
	}
	switch c.Role {
	case ws.RoleBoard:
		// Replay who has already locked in, so a board refresh mid-question
		// comes back with the right names lit rather than a blank row.
		if g.QuestionState == "active" && g.CurrentQuestionID != nil {
			ans, err := s.DB.AnswersForQuestion(ctx, *g.CurrentQuestionID)
			if err != nil {
				log.Printf("ws board join answers for question %s: %v", *g.CurrentQuestionID, err)
			} else {
				ids := make([]string, 0, len(ans))
				for _, a := range ans {
					ids = append(ids, a.UserID)
				}
				c.Send(map[string]any{
					"type": "answeredSnapshot",
					"data": map[string]any{"questionId": *g.CurrentQuestionID, "userIds": ids},
				})
			}
		}
	case ws.RoleAdmin:
		qs, err := s.DB.ListQuestions(ctx, c.GameID, true)
		if err != nil {
			log.Printf("ws join list questions for %s: %v", c.GameID, err)
		} else {
			c.Send(map[string]any{"type": "questionsAdmin", "data": qs})
		}
		c.Send(s.presenceEnvelope(c.GameID))
	case ws.RolePlayer:
		s.broadcastPresence(c.GameID)
	}
}

func (s *Server) onWSLeave(c *ws.Client) {
	if s.Metrics != nil {
		s.Metrics.WSDisconnect(string(c.Role))
	}
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
		log.Printf("ws answer game lookup for %s: %v", c.GameID, err)
		return
	}
	if g.QuestionState != "active" || g.CurrentQuestionID == nil || *g.CurrentQuestionID != m.QuestionID {
		return
	}
	q, err := s.DB.QuestionByID(ctx, m.QuestionID)
	if err != nil {
		log.Printf("ws answer question lookup for %s: %v", m.QuestionID, err)
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
	ok, pts := game.JudgeAnswer(q.AnswerType, optCount, q.Options, q.Correct, m.Value, responseMs,
		g.QuestionTimeoutSeconds*1000)
	if err := s.DB.SaveAnswer(ctx, q.ID, c.UserID, m.Value, responseMs, ok, pts); err != nil {
		log.Printf("save answer: %v", err)
	}
	if s.Metrics != nil {
		s.Metrics.RecordAnswer(ok)
	}
	// Echo personal ack to player; broadcast generic "someone answered" to admin only.
	c.Send(map[string]any{
		"type": "answerAck",
		"data": map[string]any{"questionId": q.ID, "responseMs": responseMs},
	})
	// The board lights up each team's name as they lock in, so it needs the
	// same event the admin console uses. Points are not included — only the
	// fact that this team has answered.
	s.Hub.BroadcastTo(c.GameID, map[string]any{
		"type": "playerAnswered",
		"data": map[string]any{"userId": c.UserID, "questionId": q.ID, "responseMs": responseMs},
	}, func(cl *ws.Client) bool { return cl.Role == ws.RoleAdmin || cl.Role == ws.RoleBoard })
}
