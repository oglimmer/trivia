package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolationName is the name of the per-game case-insensitive unique
// index on users(game_id, lower(name)); see migration 0009.
const uniqueViolationName = "uq_users_game_name_lower"

// mapNameTaken folds a unique-violation against the case-insensitive name
// index into the ErrNameTaken sentinel; all other errors pass through.
func mapNameTaken(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == uniqueViolationName {
		return ErrNameTaken
	}
	return err
}

// nullableID wraps a *string so the pgx driver writes UUID NULL when the
// pointer is nil; otherwise the caller's string is cast to UUID by Postgres.
func nullableID(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func (d *DB) CreateUser(ctx context.Context, gameID, name string, photoImageID *string, email, token string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO users(game_id, name, photo_image_id, email, token)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, game_id, name, photo_image_id::text, email, token, created_at, last_seen
	`, gameID, name, nullableID(photoImageID), email, token).Scan(
		&u.ID, &u.GameID, &u.Name, &u.PhotoImageID, &u.Email, &u.Token, &u.CreatedAt, &u.LastSeen)
	return u, mapNameTaken(err)
}

func (d *DB) UpdateUser(ctx context.Context, id, name string, photoImageID *string, email string) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE users SET name=$2, photo_image_id=$3, email=$4 WHERE id=$1
	`, id, name, nullableID(photoImageID), email)
	return mapNameTaken(err)
}

func (d *DB) UserByToken(ctx context.Context, token string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, game_id, name, photo_image_id::text, email, token, created_at, last_seen
		FROM users WHERE token=$1
	`, token).Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoImageID, &u.Email, &u.Token, &u.CreatedAt, &u.LastSeen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

// TouchUserLastSeen bumps the user's last_seen to now. Best-effort: callers
// log but don't surface errors to clients, since a transient write failure
// shouldn't fail the request that triggered it.
func (d *DB) TouchUserLastSeen(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE users SET last_seen = now() WHERE id=$1`, id)
	return err
}

// DeleteStaleUsers removes users in the given game whose last_seen is older
// than cutoff. Their questions and answers stay (the FK is ON DELETE SET NULL,
// see migration 0005). Returns the IDs of deleted users so callers can
// broadcast targeted updates.
func (d *DB) DeleteStaleUsers(ctx context.Context, gameID string, cutoff time.Time) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `
		DELETE FROM users WHERE game_id=$1 AND last_seen < $2 RETURNING id
	`, gameID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UserByID returns a user without its token (callers that need the token must
// use UserTokenByID).
func (d *DB) UserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, game_id, name, photo_image_id::text, email, '', created_at, last_seen
		FROM users WHERE id=$1
	`, id).Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoImageID, &u.Email, &u.Token, &u.CreatedAt, &u.LastSeen)
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

// ListUsers returns the users for a game without their tokens.
func (d *DB) ListUsers(ctx context.Context, gameID string) ([]User, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, game_id, name, photo_image_id::text, email, '', created_at, last_seen
		FROM users WHERE game_id=$1 ORDER BY created_at ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoImageID, &u.Email, &u.Token, &u.CreatedAt, &u.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AllUser is a user row enriched with the game code it belongs to, for the
// admin-wide "all registered users" listing.
type AllUser struct {
	User
	GameCode string `json:"gameCode"`
	GameName string `json:"gameName"`
}

// ListAllUsers returns every user record across all games (without tokens),
// joined with the game code/name for context. Ordered by name, then created_at.
func (d *DB) ListAllUsers(ctx context.Context) ([]AllUser, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT u.id, u.game_id, u.name, u.photo_image_id::text, u.email, '', u.created_at, u.last_seen, g.code, g.name
		FROM users u
		JOIN games g ON g.id = u.game_id
		ORDER BY lower(u.name) ASC, u.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AllUser{}
	for rows.Next() {
		var u AllUser
		if err := rows.Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoImageID, &u.Email, &u.Token, &u.CreatedAt, &u.LastSeen, &u.GameCode, &u.GameName); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
