package api

import (
	"context"
	"time"

	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// gameStateEnvelope builds the payload describing the current view of a game.
// For admin, the current question always includes correct/options; for
// players, correct is stripped while the question is still active.
func (s *Server) gameStateEnvelope(ctx context.Context, g *db.Game, asAdmin bool) map[string]any {
	out := map[string]any{
		"type": "gameState",
		"data": map[string]any{
			"code":                   g.Code,
			"name":                   g.Name,
			"state":                  g.State,
			"questionState":          g.QuestionState,
			"currentQuestionId":      g.CurrentQuestionID,
			"questionStartedAt":      g.QuestionStartedAt,
			"questionTimeoutSeconds": g.QuestionTimeoutSeconds,
			"scheduledAt":            g.ScheduledAt,
			// serverNow lets clients compute their clock offset vs. the server
			// so the question countdown stays accurate regardless of local clock skew.
			"serverNow": time.Now().UTC(),
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
