package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const gameCodeUniqueConstraint = "games_code_key"

const gameColumns = `id, code, name, state, current_question_id, question_state,
	question_started_at, question_closed_at, question_timeout_seconds, scheduled_at, created_at`

func scanGame(row pgx.Row, g *Game) error {
	return row.Scan(&g.ID, &g.Code, &g.Name, &g.State, &g.CurrentQuestionID,
		&g.QuestionState, &g.QuestionStartedAt, &g.QuestionClosedAt,
		&g.QuestionTimeoutSeconds, &g.ScheduledAt, &g.CreatedAt)
}

func (d *DB) CreateGame(ctx context.Context, code, name string, questionTimeoutSeconds int, scheduledAt *time.Time) (*Game, error) {
	g := &Game{}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO games(code, name, question_timeout_seconds, scheduled_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+gameColumns,
		code, name, questionTimeoutSeconds, scheduledAt)
	if err := scanGame(row, g); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == gameCodeUniqueConstraint {
			return nil, ErrCodeTaken
		}
		return nil, err
	}
	return g, nil
}

func (d *DB) GameByCode(ctx context.Context, code string) (*Game, error) {
	g := &Game{}
	row := d.Pool.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE code = $1`, code)
	if err := scanGame(row, g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return g, nil
}

func (d *DB) GameByID(ctx context.Context, id string) (*Game, error) {
	g := &Game{}
	row := d.Pool.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id = $1`, id)
	if err := scanGame(row, g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return g, nil
}

func (d *DB) ListGames(ctx context.Context) ([]Game, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+gameColumns+` FROM games ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Game
	for rows.Next() {
		var g Game
		if err := scanGame(rows, &g); err != nil {
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

func (d *DB) SetGameScheduledAt(ctx context.Context, id string, scheduledAt *time.Time) error {
	_, err := d.Pool.Exec(ctx, `UPDATE games SET scheduled_at=$1 WHERE id=$2`, scheduledAt, id)
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
