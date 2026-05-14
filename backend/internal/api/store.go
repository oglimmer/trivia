package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/oglimmer/trivia/backend/internal/db"
)

// Store is the data-access surface the HTTP/WS handlers depend on. The
// interface lives in this consumer package (rather than in db/) so that tests
// can substitute an in-memory fake without pulling in pgx. *db.DB satisfies
// it implicitly.
//
// Methods are grouped by aggregate purely for readability.
type Store interface {
	// Games
	CreateGame(ctx context.Context, code, name string, questionTimeoutSeconds int, scheduledAt *time.Time) (*db.Game, error)
	GameByCode(ctx context.Context, code string) (*db.Game, error)
	GameByID(ctx context.Context, id string) (*db.Game, error)
	ListGames(ctx context.Context) ([]db.Game, error)
	SetGameState(ctx context.Context, id, state string) error
	SetQuestionTimeout(ctx context.Context, id string, seconds int) error
	SetGameScheduledAt(ctx context.Context, id string, scheduledAt *time.Time) error
	ActiveQuestionGameIDs(ctx context.Context) ([]string, error)
	DeleteGame(ctx context.Context, id string) error
	ActivateQuestion(ctx context.Context, gameID, qID string) error
	CloseQuestion(ctx context.Context, gameID string) error
	ClearCurrentQuestion(ctx context.Context, gameID string) error

	// Users
	CreateUser(ctx context.Context, gameID, name, photoB64, email, token string) (*db.User, error)
	UpdateUser(ctx context.Context, id, name, photoB64, email string) error
	UserByToken(ctx context.Context, token string) (*db.User, error)
	DeleteUser(ctx context.Context, id string) error
	UserByID(ctx context.Context, id string) (*db.User, error)
	UserTokenByID(ctx context.Context, id string) (string, error)
	ListUsers(ctx context.Context, gameID string) ([]db.User, error)
	ListAllUsers(ctx context.Context) ([]db.AllUser, error)
	TouchUserLastSeen(ctx context.Context, id string) error
	DeleteStaleUsers(ctx context.Context, gameID string, cutoff time.Time) ([]string, error)

	// Questions
	UpsertQuestion(ctx context.Context, gameID, userID, text, photoB64, answerType string, options, correct json.RawMessage) (*db.Question, error)
	ListQuestions(ctx context.Context, gameID string, includeCorrect bool) ([]db.Question, error)
	QuestionByID(ctx context.Context, id string) (*db.Question, error)
	DeleteQuestion(ctx context.Context, id string) error
	RandomizeQuestionOrder(ctx context.Context, gameID string) error

	// Answers
	SaveAnswer(ctx context.Context, questionID, userID string, answer json.RawMessage, responseMs int, isCorrect bool, points int) error
	UpdateAnswerScore(ctx context.Context, questionID, userID string, isCorrect bool, points int) error
	AnswersForQuestion(ctx context.Context, questionID string) ([]db.Answer, error)
	Leaderboard(ctx context.Context, gameID string) ([]db.Score, error)
}

// Compile-time check that *db.DB still satisfies Store.
var _ Store = (*db.DB)(nil)
