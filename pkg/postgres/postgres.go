// Package postgres implements postgres connection.
package postgres

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	_defaultMaxPoolSize  = 1
	_defaultConnAttempts = 10
	_defaultConnTimeout  = time.Second
)

type Postgres struct {
	maxPoolSize  int
	connAttempts int
	connTimeout  time.Duration

	pool *pgxpool.Pool
}

func New(url string, opts ...Option) (*Postgres, error) {
	pg := &Postgres{
		maxPoolSize:  _defaultMaxPoolSize,
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	for _, opt := range opts {
		opt(pg)
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres - NewPostgres - pgxpool.ParseConfig: %w", err)
	}

	poolConfig.MaxConns = safeIntToInt32(pg.maxPoolSize)
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	for pg.connAttempts > 0 {
		pg.pool, err = connect(poolConfig, pg.connTimeout)
		if err == nil {
			break
		}

		time.Sleep(pg.connTimeout)

		pg.connAttempts--
	}

	if err != nil {
		return nil, fmt.Errorf("postgres - NewPostgres - connAttempts == 0: %w", err)
	}

	return pg, nil
}

// connect opens the pool and pings it: pgxpool connects lazily, so without the ping an attempt
// "succeeds" while the database is still down.
func connect(cfg *pgxpool.Config, timeout time.Duration) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
