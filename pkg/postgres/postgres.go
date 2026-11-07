// Package postgres implements postgres connection.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// _attemptTimeout bounds one connection attempt, so a database that drops packets is tried again.
	_attemptTimeout = time.Second
	// _retryInterval is the wait between two attempts.
	_retryInterval = time.Second

	// _invalidAuthorizationClass is the SQLSTATE class of a refused login: a wrong user or password,
	// or a pg_hba.conf rule. _invalidCatalogName is a database that does not exist.
	_invalidAuthorizationClass = "28"
	_invalidCatalogName        = "3D000"
)

type Postgres struct {
	pool *pgxpool.Pool
}

// New connects a pool of at most poolSize connections to the database at url. While the database
// cannot be reached it tries again every second, so a binary started beside its database waits for
// it; it gives up when ctx ends, with the last attempt's error. A database that answers and refuses
// the login, or has no database of that name, fails at once: no retry changes that answer.
func New(ctx context.Context, url string, poolSize int) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres - New - pgxpool.ParseConfig: %w", err)
	}

	cfg.MaxConns = safeIntToInt32(poolSize)
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	for {
		pool, err := connect(ctx, cfg)
		if err == nil {
			return &Postgres{pool: pool}, nil
		}

		if !retryable(err) {
			return nil, fmt.Errorf("postgres - New: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("postgres - New: %w", errors.Join(ctx.Err(), err))
		case <-time.After(_retryInterval):
		}
	}
}

// connect opens the pool and pings it: pgxpool connects lazily, so without the ping an attempt
// "succeeds" while the database is still down.
func connect(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, _attemptTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("pool.Ping: %w", err)
	}

	return pool, nil
}

// retryable reports whether a failed attempt may succeed later. Anything but the database's own
// answer may: a refused or dropped connection, a server still starting, too many connections.
func retryable(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return true
	}

	return !strings.HasPrefix(pgErr.Code, _invalidAuthorizationClass) && pgErr.Code != _invalidCatalogName
}

func (p *Postgres) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

// Connect opens a connection of its own, outside the pool and with its settings, for a caller that
// holds one for long, such as a LISTEN. The caller closes it.
func (p *Postgres) Connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.ConnectConfig(ctx, p.pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, fmt.Errorf("postgres - Connect: %w", err)
	}

	return conn, nil
}

// safeIntToInt32 clamps a pool size to what pgxpool takes: at least one connection, and at most
// the int32 range.
func safeIntToInt32(v int) int32 {
	return int32(min(max(v, 1), math.MaxInt32))
}
