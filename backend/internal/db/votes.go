package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// SaveVote records a single best-question vote for a player. The
// (game_id, user_id) unique constraint enforces one vote per player per game,
// so a second attempt is a no-op — votes are final once cast. Returns true when
// this call actually inserted the vote, false when the player had already voted.
func (d *DB) SaveVote(ctx context.Context, gameID, questionID, userID string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `
		INSERT INTO question_votes(game_id, question_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (game_id, user_id) DO NOTHING
	`, gameID, questionID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// VoteCounts returns vote totals keyed by question_id for a game. Questions
// with no votes are absent from the map.
func (d *DB) VoteCounts(ctx context.Context, gameID string) (map[string]int, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT question_id::text, COUNT(*)::int
		FROM question_votes WHERE game_id=$1
		GROUP BY question_id
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var qid string
		var n int
		if err := rows.Scan(&qid, &n); err != nil {
			return nil, err
		}
		out[qid] = n
	}
	return out, rows.Err()
}

// VoteCountForQuestion returns the number of votes a single question has.
func (d *DB) VoteCountForQuestion(ctx context.Context, questionID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM question_votes WHERE question_id=$1
	`, questionID).Scan(&n)
	return n, err
}

// UserVote returns the question the player voted for, or "" if they have not
// voted yet.
func (d *DB) UserVote(ctx context.Context, gameID, userID string) (string, error) {
	var qid string
	err := d.Pool.QueryRow(ctx, `
		SELECT question_id::text FROM question_votes WHERE game_id=$1 AND user_id=$2
	`, gameID, userID).Scan(&qid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return qid, err
}
