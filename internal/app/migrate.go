//go:build migrate

package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"ariga.io/atlas/atlasexec"
)

//go:embed schema
var schemaFS embed.FS

const (
	_defaultAttempts = 20
	_defaultTimeout  = time.Second
)

func init() {
	databaseURL, ok := os.LookupEnv("PG_URL")
	if !ok || len(databaseURL) == 0 {
		log.Fatalf("migrate: environment variable not declared: PG_URL")
	}
	databaseURL += "?sslmode=disable"

	devURL, ok := os.LookupEnv("ATLAS_DEV_URL")
	if !ok || len(devURL) == 0 {
		log.Fatalf("migrate: environment variable not declared: ATLAS_DEV_URL")
	}

	var (
		attempts = _defaultAttempts
		err      error
	)

	for attempts > 0 {
		err = applySchema(databaseURL, devURL)
		if err == nil {
			break
		}

		log.Printf("Migrate: postgres is trying to connect, attempts left: %d", attempts)
		time.Sleep(_defaultTimeout)
		attempts--
	}

	if err != nil {
		log.Fatalf("Migrate: schema apply error: %s", err)
	}

	log.Printf("Migrate: schema is synced")
}

func applySchema(databaseURL, devURL string) error {
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

	if _, err = client.SchemaApply(context.Background(), &atlasexec.SchemaApplyParams{
		URL:         databaseURL,
		To:          "file://schema",
		DevURL:      devURL,
		AutoApprove: true,
	}); err != nil {
		return fmt.Errorf("migrate - client.SchemaApply: %w", err)
	}

	return nil
}
