package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

type Migrator struct {
	db       *sql.DB
	provider *goose.Provider
}

type Status struct {
	Version int64
	Name    string
	Applied bool
}

func Open(ctx context.Context, url string, fsys fs.FS, l *slog.Logger) (*Migrator, error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("migrate - Open - pgx.ParseConfig: %w", err)
	}

	db := stdlib.OpenDB(*cfg)

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("migrate - Open - ping: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("migrate - Open - lock.NewPostgresSessionLocker: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithSessionLocker(locker), goose.WithSlog(l))
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("migrate - Open - goose.NewProvider: %w", err)
	}

	return &Migrator{db: db, provider: provider}, nil
}

func (m *Migrator) Up(ctx context.Context) error {
	if _, err := m.provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate - Up: %w", err)
	}

	return nil
}

func (m *Migrator) DownTo(ctx context.Context, version int64) error {
	if _, err := m.provider.DownTo(ctx, version); err != nil {
		return fmt.Errorf("migrate - DownTo: %w", err)
	}

	return nil
}

func (m *Migrator) Version(ctx context.Context) (int64, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate - Version: %w", err)
	}

	return v, nil
}

func (m *Migrator) Status(ctx context.Context) ([]Status, error) {
	all, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate - Status: %w", err)
	}

	out := make([]Status, 0, len(all))
	for _, s := range all {
		out = append(out, Status{
			Version: s.Source.Version,
			Name:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}

	return out, nil
}

func (m *Migrator) Close() error {
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("migrate - Close: %w", err)
	}

	return nil
}
