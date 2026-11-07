package projection

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

const _reconnectDelay = time.Second

// Reader reads the event store in position order.
type Reader interface {
	ReadAll(ctx context.Context, after int64, limit int) ([]entity.RecordedEvent, error)
}

// PostgresStore is the runner's Store on the app's database. Its notifications come from a
// connection of its own, outside the pool, that LISTENs on the channel every append notifies.
type PostgresStore struct {
	pg     *postgres.Postgres
	reader Reader
	logger *slog.Logger

	once   sync.Once
	notify chan struct{}
	stop   context.CancelFunc
	done   chan struct{}
}

var _ Store = (*PostgresStore)(nil)

// NewStore returns the runner's store on pg, reading events through reader.
func NewStore(pg *postgres.Postgres, reader Reader, l *slog.Logger) *PostgresStore {
	return &PostgresStore{pg: pg, reader: reader, logger: l, notify: make(chan struct{}, 1), done: make(chan struct{})}
}

// ReadAll reads the store after a position.
func (s *PostgresStore) ReadAll(ctx context.Context, after int64, limit int) ([]entity.RecordedEvent, error) {
	return s.reader.ReadAll(ctx, after, limit)
}

// InTx runs fn in one transaction.
func (s *PostgresStore) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.pg.InTx(ctx, fn)
}

// EnsureCheckpoint creates name's checkpoint at 0 if it has none.
func (s *PostgresStore) EnsureCheckpoint(ctx context.Context, name string) error {
	return queries.New(s.pg.Executor(ctx)).EnsureCheckpoint(ctx, name)
}

// Checkpoint returns name's checkpoint.
func (s *PostgresStore) Checkpoint(ctx context.Context, name string) (int64, error) {
	return queries.New(s.pg.Executor(ctx)).GetCheckpoint(ctx, name)
}

// TryLock takes name's lock until the context's transaction ends.
func (s *PostgresStore) TryLock(ctx context.Context, name string) (bool, error) {
	return queries.New(s.pg.Executor(ctx)).TryLockSubscriber(ctx, name)
}

// AdvanceCheckpoint moves name from `from` to `to` in the context's transaction.
func (s *PostgresStore) AdvanceCheckpoint(ctx context.Context, name string, from, to int64) (bool, error) {
	n, err := queries.New(s.pg.Executor(ctx)).AdvanceCheckpoint(ctx, queries.AdvanceCheckpointParams{
		Name: name, FromPosition: from, ToPosition: to,
	})

	return n == 1, err
}

// Notifications starts listening on first use and receives a value after each append commits.
func (s *PostgresStore) Notifications() <-chan struct{} {
	s.once.Do(func() {
		ctx, stop := context.WithCancel(context.Background())
		s.stop = stop

		go s.listen(ctx)
	})

	return s.notify
}

// Close stops listening.
func (s *PostgresStore) Close() {
	if s.stop == nil {
		return
	}

	s.stop()
	<-s.done
}

func (s *PostgresStore) listen(ctx context.Context) {
	defer close(s.done)

	for ctx.Err() == nil {
		if err := s.listenOnce(ctx); err != nil && ctx.Err() == nil {
			s.logger.Warn("projection - listen failed; polling until it reconnects", "error", err)

			select {
			case <-ctx.Done():
			case <-time.After(_reconnectDelay):
			}
		}
	}
}

// listenOnce holds one LISTEN connection until it fails or ctx ends.
func (s *PostgresStore) listenOnce(ctx context.Context) error {
	conn, err := pgx.ConnectConfig(ctx, s.pg.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return fmt.Errorf("pgx.ConnectConfig: %w", err)
	}

	defer func() {
		if closeErr := conn.Close(context.WithoutCancel(ctx)); closeErr != nil {
			s.logger.Warn("projection - close the listener", "error", closeErr)
		}
	}()

	if _, err = conn.Exec(ctx, "LISTEN events"); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	for {
		if _, err = conn.WaitForNotification(ctx); err != nil {
			return fmt.Errorf("WaitForNotification: %w", err)
		}

		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
}
