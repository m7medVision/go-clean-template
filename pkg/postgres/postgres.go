// Package postgres implements postgres connection.
package postgres

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	_defaultMaxPoolSize  = 1
	_defaultConnAttempts = 10
	_defaultConnTimeout  = time.Second
)

// Postgres -.
type Postgres struct {
	maxPoolSize  int
	connAttempts int
	connTimeout  time.Duration

	Pool *pgxpool.Pool
}

// New -.
func New(url string, opts ...Option) (*Postgres, error) {
	pg := &Postgres{
		maxPoolSize:  _defaultMaxPoolSize,
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	// Custom options
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
		pg.Pool, err = connect(poolConfig, pg.connTimeout)
		if err == nil {
			break
		}

		log.Printf("Postgres is trying to connect, attempts left: %d", pg.connAttempts)

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

// Close -.
func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

func safeIntToInt32(v int) int32 {
	clamped := v & math.MaxInt32

	return int32(clamped)
}
