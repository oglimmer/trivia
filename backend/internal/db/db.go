package db

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context) (*DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		envOr("POSTGRES_USER", "trivia"),
		envOr("POSTGRES_PASSWORD", "trivia"),
		envOr("POSTGRES_HOST", "localhost"),
		envOr("POSTGRES_PORT", "5432"),
		envOr("POSTGRES_DB", "trivia"),
	)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }

// Migrate runs every .sql file in dir alphabetically. Statements use
// IF NOT EXISTS, so re-running is safe.
func (d *DB) Migrate(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := os.ReadFile(dir + "/" + n)
		if err != nil {
			return err
		}
		if _, err := d.Pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("migration %s: %w", n, err)
		}
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
