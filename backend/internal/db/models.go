package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Game struct {
	ID                     string     `json:"id"`
	Code                   string     `json:"code"`
	Name                   string     `json:"name"`
	State                  string     `json:"state"`
	CurrentQuestionID      *string    `json:"currentQuestionId,omitempty"`
	QuestionState          string     `json:"questionState"`
	QuestionStartedAt      *time.Time `json:"questionStartedAt,omitempty"`
	QuestionClosedAt       *time.Time `json:"questionClosedAt,omitempty"`
	QuestionTimeoutSeconds int        `json:"questionTimeoutSeconds"`
	CreatedAt              time.Time  `json:"createdAt"`
}

type User struct {
	ID        string    `json:"id"`
	GameID    string    `json:"gameId"`
	Name      string    `json:"name"`
	PhotoB64  string    `json:"photoB64,omitempty"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Question struct {
	ID         string          `json:"id"`
	GameID     string          `json:"gameId"`
	UserID     string          `json:"userId"`
	Text       string          `json:"text"`
	PhotoB64   string          `json:"photoB64,omitempty"`
	AnswerType string          `json:"answerType"`
	Options    json.RawMessage `json:"options"`
	Correct    json.RawMessage `json:"correct,omitempty"`
	SortOrder  int             `json:"sortOrder"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type Answer struct {
	ID         string          `json:"id"`
	QuestionID string          `json:"questionId"`
	UserID     string          `json:"userId"`
	Answer     json.RawMessage `json:"answer"`
	ResponseMs int             `json:"responseMs"`
	IsCorrect  bool            `json:"isCorrect"`
	Points     int             `json:"points"`
	CreatedAt  time.Time       `json:"createdAt"`
}

var ErrNotFound = errors.New("not found")

// ---------- Games ----------

func (d *DB) CreateGame(ctx context.Context, code, name string, questionTimeoutSeconds int) (*Game, error) {
	g := &Game{}
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO games(code, name, question_timeout_seconds)
		VALUES ($1, $2, $3)
		RETURNING id, code, name, state, current_question_id, question_state,
		          question_started_at, question_closed_at, question_timeout_seconds, created_at
	`, code, name, questionTimeoutSeconds).Scan(&g.ID, &g.Code, &g.Name, &g.State, &g.CurrentQuestionID,
		&g.QuestionState, &g.QuestionStartedAt, &g.QuestionClosedAt, &g.QuestionTimeoutSeconds, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (d *DB) GameByCode(ctx context.Context, code string) (*Game, error) {
	g := &Game{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, code, name, state, current_question_id, question_state,
		       question_started_at, question_closed_at, question_timeout_seconds, created_at
		FROM games WHERE code = $1
	`, code).Scan(&g.ID, &g.Code, &g.Name, &g.State, &g.CurrentQuestionID,
		&g.QuestionState, &g.QuestionStartedAt, &g.QuestionClosedAt, &g.QuestionTimeoutSeconds, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

func (d *DB) GameByID(ctx context.Context, id string) (*Game, error) {
	g := &Game{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, code, name, state, current_question_id, question_state,
		       question_started_at, question_closed_at, question_timeout_seconds, created_at
		FROM games WHERE id = $1
	`, id).Scan(&g.ID, &g.Code, &g.Name, &g.State, &g.CurrentQuestionID,
		&g.QuestionState, &g.QuestionStartedAt, &g.QuestionClosedAt, &g.QuestionTimeoutSeconds, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

func (d *DB) ListGames(ctx context.Context) ([]Game, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, code, name, state, current_question_id, question_state,
		       question_started_at, question_closed_at, question_timeout_seconds, created_at
		FROM games ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.State, &g.CurrentQuestionID,
			&g.QuestionState, &g.QuestionStartedAt, &g.QuestionClosedAt, &g.QuestionTimeoutSeconds, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) SetGameState(ctx context.Context, id, state string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE games SET state=$1 WHERE id=$2`, state, id)
	return err
}

func (d *DB) SetQuestionTimeout(ctx context.Context, id string, seconds int) error {
	_, err := d.Pool.Exec(ctx, `UPDATE games SET question_timeout_seconds=$1 WHERE id=$2`, seconds, id)
	return err
}

// ActiveQuestionGameIDs returns ids of games that currently have an active
// question. Used at startup to re-arm auto-close timers.
func (d *DB) ActiveQuestionGameIDs(ctx context.Context) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id FROM games WHERE question_state='active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (d *DB) DeleteGame(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM games WHERE id=$1`, id)
	return err
}

func (d *DB) ActivateQuestion(ctx context.Context, gameID, qID string) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE games
		SET current_question_id=$2, question_state='active',
		    question_started_at=now(), question_closed_at=NULL
		WHERE id=$1
	`, gameID, qID)
	return err
}

func (d *DB) CloseQuestion(ctx context.Context, gameID string) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE games
		SET question_state='revealed', question_closed_at=now()
		WHERE id=$1
	`, gameID)
	return err
}

func (d *DB) ClearCurrentQuestion(ctx context.Context, gameID string) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE games
		SET current_question_id=NULL, question_state='idle',
		    question_started_at=NULL, question_closed_at=NULL
		WHERE id=$1
	`, gameID)
	return err
}

// ---------- Users ----------

func (d *DB) CreateUser(ctx context.Context, gameID, name, photoB64, token string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO users(game_id, name, photo_b64, token)
		VALUES ($1, $2, $3, $4)
		RETURNING id, game_id, name, photo_b64, token, created_at
	`, gameID, name, photoB64, token).Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoB64, &u.Token, &u.CreatedAt)
	return u, err
}

func (d *DB) UpdateUser(ctx context.Context, id, name, photoB64 string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE users SET name=$2, photo_b64=$3 WHERE id=$1`, id, name, photoB64)
	return err
}

func (d *DB) UserByToken(ctx context.Context, token string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, game_id, name, photo_b64, token, created_at
		FROM users WHERE token=$1
	`, token).Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoB64, &u.Token, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

func (d *DB) UserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, game_id, name, photo_b64, '', created_at
		FROM users WHERE id=$1
	`, id).Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoB64, &u.Token, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (d *DB) UserTokenByID(ctx context.Context, id string) (string, error) {
	var tok string
	err := d.Pool.QueryRow(ctx, `SELECT token FROM users WHERE id=$1`, id).Scan(&tok)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return tok, err
}

func (d *DB) ListUsers(ctx context.Context, gameID string) ([]User, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, game_id, name, photo_b64, '', created_at
		FROM users WHERE game_id=$1 ORDER BY created_at ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoB64, &u.Token, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ---------- Questions ----------

func (d *DB) UpsertQuestion(ctx context.Context, gameID, userID, text, photoB64, answerType string, options, correct json.RawMessage) (*Question, error) {
	q := &Question{}
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO questions(game_id, user_id, text, photo_b64, answer_type, options, correct)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (game_id, user_id) DO UPDATE
		SET text=EXCLUDED.text, photo_b64=EXCLUDED.photo_b64,
		    answer_type=EXCLUDED.answer_type, options=EXCLUDED.options, correct=EXCLUDED.correct
		RETURNING id, game_id, user_id, text, photo_b64, answer_type, options, correct, sort_order, created_at
	`, gameID, userID, text, photoB64, answerType, options, correct).Scan(
		&q.ID, &q.GameID, &q.UserID, &q.Text, &q.PhotoB64, &q.AnswerType,
		&q.Options, &q.Correct, &q.SortOrder, &q.CreatedAt,
	)
	return q, err
}

func (d *DB) ListQuestions(ctx context.Context, gameID string, includeCorrect bool) ([]Question, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, game_id, user_id, text, photo_b64, answer_type, options, correct, sort_order, created_at
		FROM questions WHERE game_id=$1 ORDER BY sort_order, created_at ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Question{}
	for rows.Next() {
		var q Question
		if err := rows.Scan(&q.ID, &q.GameID, &q.UserID, &q.Text, &q.PhotoB64,
			&q.AnswerType, &q.Options, &q.Correct, &q.SortOrder, &q.CreatedAt); err != nil {
			return nil, err
		}
		if !includeCorrect {
			q.Correct = nil
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (d *DB) QuestionByID(ctx context.Context, id string) (*Question, error) {
	q := &Question{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, game_id, user_id, text, photo_b64, answer_type, options, correct, sort_order, created_at
		FROM questions WHERE id=$1
	`, id).Scan(&q.ID, &q.GameID, &q.UserID, &q.Text, &q.PhotoB64,
		&q.AnswerType, &q.Options, &q.Correct, &q.SortOrder, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return q, err
}

func (d *DB) DeleteQuestion(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM questions WHERE id=$1`, id)
	return err
}

func (d *DB) RandomizeQuestionOrder(ctx context.Context, gameID string) error {
	_, err := d.Pool.Exec(ctx, `
		WITH shuffled AS (
		  SELECT id, row_number() OVER (ORDER BY random()) AS rn
		  FROM questions WHERE game_id = $1
		)
		UPDATE questions q SET sort_order = s.rn FROM shuffled s WHERE q.id = s.id
	`, gameID)
	return err
}

// ---------- Answers ----------

func (d *DB) SaveAnswer(ctx context.Context, questionID, userID string, answer json.RawMessage, responseMs int, isCorrect bool, points int) error {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO answers(question_id, user_id, answer, response_ms, is_correct, points)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (question_id, user_id) DO NOTHING
	`, questionID, userID, answer, responseMs, isCorrect, points)
	return err
}

func (d *DB) UpdateAnswerScore(ctx context.Context, questionID, userID string, isCorrect bool, points int) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE answers SET is_correct=$3, points=$4
		WHERE question_id=$1 AND user_id=$2
	`, questionID, userID, isCorrect, points)
	return err
}

func (d *DB) AnswersForQuestion(ctx context.Context, questionID string) ([]Answer, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, question_id, user_id, answer, response_ms, is_correct, points, created_at
		FROM answers WHERE question_id=$1 ORDER BY response_ms ASC
	`, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Answer
	for rows.Next() {
		var a Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.UserID, &a.Answer,
			&a.ResponseMs, &a.IsCorrect, &a.Points, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type Score struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	PhotoB64 string `json:"photoB64,omitempty"`
	Points   int    `json:"points"`
	Correct  int    `json:"correct"`
}

func (d *DB) Leaderboard(ctx context.Context, gameID string) ([]Score, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT u.id, u.name, u.photo_b64,
		       COALESCE(SUM(a.points), 0)::INT AS points,
		       COALESCE(SUM(CASE WHEN a.is_correct THEN 1 ELSE 0 END), 0)::INT AS correct
		FROM users u
		LEFT JOIN answers a ON a.user_id = u.id
		LEFT JOIN questions q ON q.id = a.question_id AND q.game_id = u.game_id
		WHERE u.game_id = $1
		GROUP BY u.id, u.name, u.photo_b64
		ORDER BY points DESC, u.name ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Score
	for rows.Next() {
		var s Score
		if err := rows.Scan(&s.UserID, &s.UserName, &s.PhotoB64, &s.Points, &s.Correct); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
