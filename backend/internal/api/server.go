package api

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/oglimmer/trivia/backend/internal/ai"
	"github.com/oglimmer/trivia/backend/internal/game"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// Server is the HTTP API plus the live WebSocket hub.
type Server struct {
	DB  Store
	Hub *ws.Hub
	AI  *ai.Client

	// gameLocks serializes admin transitions per game.
	mu        sync.Mutex
	gameLocks map[string]*sync.Mutex

	// autoClose holds the pending question-timeout timer per game, so we can
	// cancel it if the admin reveals/advances first.
	autoCloseMu sync.Mutex
	autoClose   map[string]*time.Timer
}

func New(d Store, h *ws.Hub, c *ai.Client) *Server {
	s := &Server{
		DB: d, Hub: h, AI: c,
		gameLocks: map[string]*sync.Mutex{},
		autoClose: map[string]*time.Timer{},
	}
	h.OnRecv = s.onWSMessage
	h.OnJoin = s.onWSJoin
	h.OnLeave = s.onWSLeave
	return s
}

// ResumeAutoCloseTimers re-arms pending auto-close timers at startup so a
// server restart doesn't leave players stuck on an expired question.
func (s *Server) ResumeAutoCloseTimers(ctx context.Context) {
	ids, err := s.DB.ActiveQuestionGameIDs(ctx)
	if err != nil {
		log.Printf("resume auto-close: %v", err)
		return
	}
	for _, id := range ids {
		g, err := s.DB.GameByID(ctx, id)
		if err != nil || g.QuestionStartedAt == nil || g.CurrentQuestionID == nil || g.QuestionTimeoutSeconds <= 0 {
			continue
		}
		deadline := g.QuestionStartedAt.Add(time.Duration(g.QuestionTimeoutSeconds) * time.Second)
		d := time.Until(deadline)
		if d < 0 {
			d = 0
		}
		s.scheduleAutoClose(id, *g.CurrentQuestionID, d)
	}
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

func (s *Server) dropGameLock(gameID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.gameLocks, gameID)
}

func (s *Server) scheduleAutoClose(gameID, questionID string, d time.Duration) {
	s.autoCloseMu.Lock()
	defer s.autoCloseMu.Unlock()
	if t, ok := s.autoClose[gameID]; ok {
		t.Stop()
	}
	s.autoClose[gameID] = time.AfterFunc(d, func() {
		s.autoCloseFire(gameID, questionID)
	})
}

func (s *Server) cancelAutoClose(gameID string) {
	s.autoCloseMu.Lock()
	defer s.autoCloseMu.Unlock()
	if t, ok := s.autoClose[gameID]; ok {
		t.Stop()
		delete(s.autoClose, gameID)
	}
}

// autoCloseFire runs when a question's timer expires. It reveals the
// question for everyone — same effect as the admin clicking "Reveal".
func (s *Server) autoCloseFire(gameID, questionID string) {
	lock := s.lockFor(gameID)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	g, err := s.DB.GameByID(ctx, gameID)
	if err != nil {
		return
	}
	// State may have moved on (admin revealed or advanced) while we were waiting.
	if g.QuestionState != "active" || g.CurrentQuestionID == nil || *g.CurrentQuestionID != questionID {
		return
	}
	if err := s.rescoreNumberAnswers(ctx, questionID); err != nil {
		log.Printf("auto-close rescore: %v", err)
	}
	if err := s.DB.CloseQuestion(ctx, gameID); err != nil {
		log.Printf("auto-close: %v", err)
		return
	}
	s.broadcastGameState(ctx, gameID)
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
