package db

import (
	"context"
	"encoding/json"
)

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
