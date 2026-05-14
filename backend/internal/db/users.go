package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

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

// UserByID returns a user without its token (callers that need the token must
// use UserTokenByID).
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

// ListUsers returns the users for a game without their tokens.
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
		SELECT u.id, u.game_id, u.name, u.photo_b64, '', u.created_at, g.code, g.name
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
		if err := rows.Scan(&u.ID, &u.GameID, &u.Name, &u.PhotoB64, &u.Token, &u.CreatedAt, &u.GameCode, &u.GameName); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
