//go:build migrate

package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"time"

	"ariga.io/atlas/atlasexec"
)

//go:embed schema
var schemaFS embed.FS

const _applyTimeout = 5 * time.Minute

// syncSchema applies the HCL schema to the database at databaseURL. Atlas connects on its own and
// asks for TLS unless the URL says otherwise, so a URL without sslmode gets sslmode=disable.
func syncSchema(ctx context.Context, l *slog.Logger, databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("migrate - url.Parse: %w", err)
	}

	q := u.Query()
	if !q.Has("sslmode") {
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
	}

	if err := applySchema(ctx, u.String()); err != nil {
		return err
	}

	l.Info("migrate - schema applied")

	return nil
}

// applySchema diffs the HCL schema directly against the app database: no Atlas dev database, so the
// stack runs exactly one database.
func applySchema(ctx context.Context, databaseURL string) error {
	ctx, cancel := context.WithTimeout(ctx, _applyTimeout)
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
		AutoApprove: true,
	}); err != nil {
		return fmt.Errorf("migrate - client.SchemaApply: %w", err)
	}

	return nil
}
