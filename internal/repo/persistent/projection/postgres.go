package projection

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
}

var _ Store = (*PostgresStore)(nil)

func NewStore(pg *postgres.Postgres, reader Reader, l *slog.Logger) *PostgresStore {
	return &PostgresStore{pg: pg, reader: reader, logger: l}
}

func (s *PostgresStore) ReadAll(ctx context.Context, after int64, limit int) ([]entity.RecordedEvent, error) {
	return s.reader.ReadAll(ctx, after, limit)
}

func (s *PostgresStore) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.pg.InTx(ctx, fn)
}

// EnsureCheckpoint creates name's checkpoint at 0 if it has none.
func (s *PostgresStore) EnsureCheckpoint(ctx context.Context, name string) error {
	return queries.New(s.pg.Executor(ctx)).EnsureCheckpoint(ctx, name)
}

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

// Notifications listens until ctx ends, and receives a value after each append commits. It is
// closed once the listening connection is.
func (s *PostgresStore) Notifications(ctx context.Context) <-chan struct{} {
	notify := make(chan struct{}, 1)

	go s.listen(ctx, notify)

	return notify
}

func (s *PostgresStore) listen(ctx context.Context, notify chan<- struct{}) {
	defer close(notify)

	for ctx.Err() == nil {
		if err := s.listenOnce(ctx, notify); err != nil && ctx.Err() == nil {
			s.logger.Warn("projection - listen failed; polling until it reconnects", "error", err)

			select {
			case <-ctx.Done():
			case <-time.After(_reconnectDelay):
			}
		}
	}
}

func (s *PostgresStore) listenOnce(ctx context.Context, notify chan<- struct{}) error {
	conn, err := s.pg.Connect(ctx)
	if err != nil {
		return fmt.Errorf("s.pg.Connect: %w", err)
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
		case notify <- struct{}{}:
		default:
		}
	}
}
