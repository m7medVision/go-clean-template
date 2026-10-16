//go:build migrate

package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"ariga.io/atlas/atlasexec"
	"github.com/jackc/pgx/v5"
)

//go:embed schema
var schemaFS embed.FS

const (
	_defaultAttempts = 20
	_defaultTimeout  = time.Second
	_applyTimeout    = 5 * time.Minute
)

func syncSchema() {
	databaseURL, ok := os.LookupEnv("PG_URL")
	if !ok || databaseURL == "" {
		log.Fatalf("migrate: environment variable not declared: PG_URL")
	}

	databaseURL += "?sslmode=disable"

	devURL, ok := os.LookupEnv("ATLAS_DEV_URL")
	if !ok || devURL == "" {
		log.Fatalf("migrate: environment variable not declared: ATLAS_DEV_URL")
	}

	waitForPostgres(databaseURL)

	if strings.HasPrefix(devURL, "postgres://") {
		waitForPostgres(devURL)
	}

	if err := applySchema(databaseURL, devURL); err != nil {
		log.Fatalf("Migrate: schema apply error: %s", err)
	}

	log.Printf("Migrate: schema is synced")
}

func waitForPostgres(url string) {
	var err error

	for attempts := _defaultAttempts; attempts > 0; attempts-- {
		err = ping(url)
		if err == nil {
			return
		}

		log.Printf("Migrate: postgres is trying to connect, attempts left: %d", attempts)
		time.Sleep(_defaultTimeout)
	}

	log.Fatalf("Migrate: postgres connect error: %s", err)
}

func ping(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), _defaultTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("migrate - pgx.Connect: %w", err)
	}
	defer conn.Close(ctx)

	if err = conn.Ping(ctx); err != nil {
		return fmt.Errorf("migrate - conn.Ping: %w", err)
	}

	return nil
}

func applySchema(databaseURL, devURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), _applyTimeout)
	defer cancel()

	workdir, err := atlasexec.NewWorkingDir()
	if err != nil {
		return fmt.Errorf("migrate - atlasexec.NewWorkingDir: %w", err)
	}
	defer workdir.Close()

	sub, err := fs.Sub(schemaFS, "schema")
	if err != nil {
		return fmt.Errorf("migrate - fs.Sub: %w", err)
	}

	if err = workdir.CopyFS("schema", sub); err != nil {
		return fmt.Errorf("migrate - workdir.CopyFS: %w", err)
	}

	client, err := atlasexec.NewClient(workdir.Path(), "atlas")
	if err != nil {
		return fmt.Errorf("migrate - atlasexec.NewClient: %w", err)
	}

	if _, err = client.SchemaApply(ctx, &atlasexec.SchemaApplyParams{
		URL:         databaseURL,
		To:          "file://schema",
		DevURL:      devURL,
		AutoApprove: true,
	}); err != nil {
		return fmt.Errorf("migrate - client.SchemaApply: %w", err)
	}

	return nil
}
