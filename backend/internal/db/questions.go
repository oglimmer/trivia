package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

const questionColumns = `id, game_id, COALESCE(user_id::text, '') AS user_id, text, photo_image_id::text, answer_type,
	options, correct, sort_order, created_at`

func scanQuestion(row pgx.Row, q *Question) error {
	return row.Scan(&q.ID, &q.GameID, &q.UserID, &q.Text, &q.PhotoImageID,
		&q.AnswerType, &q.Options, &q.Correct, &q.SortOrder, &q.CreatedAt)
}

func (d *DB) UpsertQuestion(ctx context.Context, gameID, userID, text string, photoImageID *string, answerType string, options, correct json.RawMessage) (*Question, error) {
	q := &Question{}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO questions(game_id, user_id, text, photo_image_id, answer_type, options, correct)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (game_id, user_id) DO UPDATE
		SET text=EXCLUDED.text,
		    photo_image_id=EXCLUDED.photo_image_id,
		    answer_type=EXCLUDED.answer_type, options=EXCLUDED.options, correct=EXCLUDED.correct
		RETURNING `+questionColumns,
		gameID, userID, text, nullableID(photoImageID), answerType, options, correct)
	if err := scanQuestion(row, q); err != nil {
		return nil, err
	}
	return q, nil
}

func (d *DB) ListQuestions(ctx context.Context, gameID string, includeCorrect bool) ([]Question, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT `+questionColumns+`
		FROM questions WHERE game_id=$1 ORDER BY sort_order, created_at ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Question{}
	for rows.Next() {
		var q Question
		if err := scanQuestion(rows, &q); err != nil {
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
	row := d.Pool.QueryRow(ctx, `SELECT `+questionColumns+` FROM questions WHERE id=$1`, id)
	if err := scanQuestion(row, q); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
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

// HostQuestion is one entry of a host-authored question set (poll mode).
// There is no author: user_id stays NULL, which the UNIQUE (game_id, user_id)
// index tolerates because Postgres does not treat NULLs as equal.
type HostQuestion struct {
	Text       string
	AnswerType string
	Options    json.RawMessage
	Correct    json.RawMessage
}

// ReplaceHostQuestions swaps the game's entire host-authored question set in a
// single transaction. Player-authored questions (user_id NOT NULL) are left
// alone, so a mixed game is not silently gutted by a re-import.
//
// Replace rather than append: re-importing after a typo in the points is the
// expected workflow, and an append would quietly double the question count.
func (d *DB) ReplaceHostQuestions(ctx context.Context, gameID string, items []HostQuestion) ([]Question, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM questions WHERE game_id=$1 AND user_id IS NULL`, gameID); err != nil {
		return nil, err
	}
	out := make([]Question, 0, len(items))
	for i, it := range items {
		var q Question
		row := tx.QueryRow(ctx, `
			INSERT INTO questions(game_id, user_id, text, photo_image_id, answer_type, options, correct, sort_order)
			VALUES ($1, NULL, $2, NULL, $3, $4, $5, $6)
			RETURNING `+questionColumns,
			gameID, it.Text, it.AnswerType, it.Options, it.Correct, i+1)
		if err := scanQuestion(row, &q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateHostQuestion appends one host-authored question to a game, landing it
// at the end of the running order.
func (d *DB) CreateHostQuestion(ctx context.Context, gameID string, it HostQuestion) (*Question, error) {
	q := &Question{}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO questions(game_id, user_id, text, photo_image_id, answer_type, options, correct, sort_order)
		VALUES ($1, NULL, $2, NULL, $3, $4, $5,
		        (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM questions WHERE game_id = $1))
		RETURNING `+questionColumns,
		gameID, it.Text, it.AnswerType, it.Options, it.Correct)
	if err := scanQuestion(row, q); err != nil {
		return nil, err
	}
	return q, nil
}

// UpdateHostQuestion rewrites one host-authored question in place, preserving
// its position in the running order. It refuses to touch a player-written
// question — those belong to their author and are edited on the player's own
// setup page.
func (d *DB) UpdateHostQuestion(ctx context.Context, questionID string, it HostQuestion) (*Question, error) {
	q := &Question{}
	row := d.Pool.QueryRow(ctx, `
		UPDATE questions
		SET text=$2, answer_type=$3, options=$4, correct=$5
		WHERE id=$1 AND user_id IS NULL
		RETURNING `+questionColumns,
		questionID, it.Text, it.AnswerType, it.Options, it.Correct)
	if err := scanQuestion(row, q); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

// MoveQuestion shifts one question one slot earlier or later in the running
// order. It rewrites sort_order for the whole game as a dense 1..n sequence,
// so a set that arrived with duplicate or zeroed values is normalised on the
// first move rather than silently refusing to reorder.
func (d *DB) MoveQuestion(ctx context.Context, gameID, questionID string, delta int) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id FROM questions WHERE game_id=$1 ORDER BY sort_order, created_at ASC
	`, gameID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	from := -1
	for i, id := range ids {
		if id == questionID {
			from = i
			break
		}
	}
	if from < 0 {
		return ErrNotFound
	}
	to := from + delta
	if to < 0 || to >= len(ids) {
		// Already at the end it was asked to move past. Not an error — the
		// button is just a no-op there.
		return tx.Commit(ctx)
	}
	ids[from], ids[to] = ids[to], ids[from]

	for i, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE questions SET sort_order=$2 WHERE id=$1`, id, i+1); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
