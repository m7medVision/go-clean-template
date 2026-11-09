package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/m7medvision/go-clean-template/internal/app/migrations"
	"github.com/m7medvision/go-clean-template/pkg/migrate"
	"github.com/pressly/goose/v3"
)

const (
	_migrationsDir = "internal/app/migrations"
	_maxArgs       = 2
)

var (
	errUsage    = errors.New("usage: migrate [-url URL] [-dir DIR] up | down-to VERSION | status | version | create NAME")
	errNotEmpty = errors.New("the database already has migrations applied; round-trip needs a throwaway one")
)

func main() {
	url := flag.String("url", os.Getenv("PG_URL"), "the database URL")
	dir := flag.String("dir", "", "read migrations from this directory instead of the ones built in")

	flag.Parse()

	l := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, l, *url, *dir, flag.Args())

	stop()

	if err != nil {
		l.Error("migrate", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, l *slog.Logger, url, dir string, args []string) error {
	if len(args) == 0 {
		return errUsage
	}

	name, arg := args[0], ""
	if len(args) > 1 {
		arg = args[1]
	}

	if len(args) > _maxArgs || (needsArg(name) != (arg != "")) {
		return errUsage
	}

	if name == "create" {
		if err := goose.Create(nil, _migrationsDir, arg, "sql"); err != nil {
			return fmt.Errorf("create: %w", err)
		}

		return nil
	}

	var fsys fs.FS = migrations.FS
	if dir != "" {
		fsys = os.DirFS(dir)
	}

	m, err := migrate.Open(ctx, url, fsys, l)
	if err != nil {
		return err
	}
	defer m.Close()

	return command(ctx, m, name, arg)
}

func needsArg(name string) bool {
	return name == "create" || name == "down-to"
}

func command(ctx context.Context, m *migrate.Migrator, name, arg string) error {
	switch name {
	case "up":
		return m.Up(ctx)
	case "down-to":
		v, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return fmt.Errorf("down-to: %w", err)
		}

		return m.DownTo(ctx, v)
	case "status":
		return status(ctx, m)
	case "version":
		v, err := m.Version(ctx)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(os.Stdout, v)

		return err
	case "require-empty":
		return requireEmpty(ctx, m)
	default:
		return errUsage
	}
}

func requireEmpty(ctx context.Context, m *migrate.Migrator) error {
	v, err := m.Version(ctx)
	if err != nil {
		return err
	}

	if v != 0 {
		return errNotEmpty
	}

	return nil
}

func status(ctx context.Context, m *migrate.Migrator) error {
	all, err := m.Status(ctx)
	if err != nil {
		return err
	}

	for _, s := range all {
		state := "pending"
		if s.Applied {
			state = "applied"
		}

		if _, err = fmt.Fprintf(os.Stdout, "%-8s %d %s\n", state, s.Version, s.Name); err != nil {
			return err
		}
	}

	return nil
}
