package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store implements repo.Store backed by PostgreSQL via pgxpool.
type Store struct {
	pool *pgxpool.Pool
	url  string
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	s := &Store{pool: pool, url: databaseURL}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

// migrate applies embedded SQL migrations in filename order using the simple
// protocol so multiple statements per file are allowed.
func (s *Store) migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		raw, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}

		cfg, err := pgx.ParseConfig(s.url)
		if err != nil {
			return err
		}
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			return err
		}
		_, execErr := conn.Exec(ctx, string(raw))
		closeErr := conn.Close(ctx)
		if execErr != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), execErr)
		}
		if closeErr != nil {
			return fmt.Errorf("migration %s close: %w", e.Name(), closeErr)
		}
	}
	return nil
}
