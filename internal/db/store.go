package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/effiware/goth-template/internal/version"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"go.opentelemetry.io/otel"
)

const gooseTableName = "tech_goose_db_version" // local tool uses GOOSE_TABLE from .env

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	pool *pgxpool.Pool
	Querier
}

func NewStore(ctx context.Context, dbURL string, maxConns int) (*Store, error) {
	pool, err := NewPgPool(ctx, dbURL, maxConns)
	if err != nil {
		return nil, fmt.Errorf("DB connection pool: %w", err)
	}

	return &Store{
		pool:    pool,
		Querier: New(pool),
	}, nil
}

// PoolMinConns is the floor below which background ticks starve request handling —
// each gated service run pins a connection for its session lock.
const PoolMinConns = 6

// NewPgPool creates connection pool to Postgres DB. maxConns <= 0 keeps the DSN's own
// pool_max_conns (or pgx's default).
func NewPgPool(ctx context.Context, connString string, maxConns int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing pool config failed: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = int32(maxConns)
	}
	cfg.ConnConfig.Tracer = poolTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool failed: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database failed: %w", err)
	}

	effective := pool.Config().MaxConns
	if effective < PoolMinConns {
		slog.Warn("Postgres pool below the recommended floor — background ticks can starve request handling; set database.pool_max_conns",
			"max_conns", effective, "recommended_min", PoolMinConns)
	}
	slog.Info("Postgres pool ready", "max_conns", effective)

	return pool, nil
}

// Ping reports whether the pool can reach Postgres; the readiness probe's DB half.
func (s *Store) Ping(ctx context.Context) error {
	if s.pool == nil {
		return nil // pool-less test double — nothing to reach
	}
	return s.pool.Ping(ctx)
}

// WithTx is high level wrapper over (simple) db transaction management.
//
// A store with no pool (a test double wrapping a fake Querier) has no real
// transaction to open, so fn runs directly against the embedded Querier.
func (s *Store) WithTx(ctx context.Context, fn func(Querier) error) error {
	if s.pool == nil {
		return fn(s.Querier)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TryWithSessionLock runs fn while holding the session-scoped advisory lock
// key, acquired non-blockingly on a dedicated pooled connection that stays
// checked out for the whole run (session advisory locks are per-connection).
// If the holding instance dies mid-run, Postgres releases the lock with the
// dead connection — Caveat: breaks behind a transaction-mode pooler
// (pgbouncer); we connect via pgxpool directly.
//
// A pool-less store has no peer instances to exclude, so fn runs directly.
func (s *Store) TryWithSessionLock(ctx context.Context, key int64, fn func() error) (bool, error) {
	if s.pool == nil {
		return true, fn()
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire connection for session lock: %w", err)
	}
	defer conn.Release()

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		return false, fmt.Errorf("try advisory lock: %w", err)
	}
	if !acquired {
		return false, nil
	}
	defer func() {
		// WithoutCancel: a canceled ctx must not skip the unlock — the conn goes
		// back to the pool and would otherwise keep holding the lock. If the
		// unlock itself fails the connection is destroyed instead, which releases
		// the lock server-side.
		if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", key); err != nil {
			_ = conn.Conn().Close(context.WithoutCancel(ctx))
		}
	}()
	return true, fn()
}

// Close releases the connection pool. The server holds its store for the whole
// process lifetime and never needs this; a subcommand that returns does.
// Safe on a pool-less store.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// RunMigrations runs goose migrations against the DB
func (s *Store) RunMigrations(ctx context.Context) error {
	// One parent span so the pool tracer's goose queries nest instead of emitting root spans.
	ctx, span := otel.Tracer(version.ServiceName).Start(ctx, "db.RunMigrations")
	defer span.End()

	migrationsFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("sub migrations fs: %w", err)
	}

	db := stdlib.OpenDBFromPool(s.pool)

	// Postgres advisory session lock: when several app instances boot with pending migrations
	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration session locker: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS,
		goose.WithTableName(gooseTableName),
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
