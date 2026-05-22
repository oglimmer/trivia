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

	qs, _ := s.DB.ListQuestions(ctx, g.ID, true)
	data["totalQuestions"] = len(qs)
	questionIndex := 0
	var current *db.Question
	if g.CurrentQuestionID != nil {
		for i := range qs {
			if qs[i].ID == *g.CurrentQuestionID {
				questionIndex = i + 1
				current = &qs[i]
				break
			}
		}
	}
	data["questionIndex"] = questionIndex

	if current != nil {
		qd := map[string]any{
			"id":           current.ID,
			"text":         current.Text,
			"photoImageId": current.PhotoImageID,
			"answerType":   current.AnswerType,
			"options":      current.Options,
			"userId":       current.UserID,
		}
		if asAdmin || g.QuestionState == "revealed" {
			qd["correct"] = current.Correct
		}
		data["question"] = qd
		if g.QuestionState == "revealed" {
			ans, _ := s.DB.AnswersForQuestion(ctx, current.ID)
			data["answers"] = ans
		}
	}

	if g.State == "finished" || g.QuestionState == "revealed" {
		hidden := !asAdmin && inLeaderboardSuspense(g, len(qs), questionIndex)
		if hidden {
			data["leaderboardHidden"] = true
		} else {
			lb, _ := s.DB.Leaderboard(ctx, g.ID)
			data["leaderboard"] = lb
		}
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
