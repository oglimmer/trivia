package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

const questionColumns = `id, game_id, COALESCE(user_id::text, '') AS user_id, text, photo_b64, answer_type,
	options, correct, sort_order, created_at`

func scanQuestion(row pgx.Row, q *Question) error {
	return row.Scan(&q.ID, &q.GameID, &q.UserID, &q.Text, &q.PhotoB64,
		&q.AnswerType, &q.Options, &q.Correct, &q.SortOrder, &q.CreatedAt)
}

func (d *DB) UpsertQuestion(ctx context.Context, gameID, userID, text, photoB64, answerType string, options, correct json.RawMessage) (*Question, error) {
	q := &Question{}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO questions(game_id, user_id, text, photo_b64, answer_type, options, correct)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (game_id, user_id) DO UPDATE
		SET text=EXCLUDED.text, photo_b64=EXCLUDED.photo_b64,
		    answer_type=EXCLUDED.answer_type, options=EXCLUDED.options, correct=EXCLUDED.correct
		RETURNING `+questionColumns,
		gameID, userID, text, photoB64, answerType, options, correct)
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
