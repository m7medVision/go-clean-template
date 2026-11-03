package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errImport = errors.New("import failed")

// countingImporter counts its runs, and fails while fail is set.
type countingImporter struct {
	domain string
	runs   int
	fail   bool
}

func (c *countingImporter) Domain() string { return c.domain }

func (c *countingImporter) Import(context.Context) error {
	c.runs++
	if c.fail {
		return errImport
	}

	return nil
}

func TestBackfillRunsEachImporterOnce(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	imp := &countingImporter{domain: "_test-" + uuid.NewString()}

	require.NoError(t, eventstore.Backfill(ctx, pg, imp))
	require.NoError(t, eventstore.Backfill(ctx, pg, imp))

	assert.Equal(t, 1, imp.runs)

	marked, err := queries.New(pg.Pool).HasMarker(ctx, "_backfill/"+imp.domain)
	require.NoError(t, err)
	assert.True(t, marked)
}

func TestAFailedImportLeavesNoMarkerAndRunsAgain(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	imp := &countingImporter{domain: "_test-" + uuid.NewString(), fail: true}

	require.ErrorIs(t, eventstore.Backfill(ctx, pg, imp), errImport)

	marked, err := queries.New(pg.Pool).HasMarker(ctx, "_backfill/"+imp.domain)
	require.NoError(t, err)
	assert.False(t, marked)

	imp.fail = false
	require.NoError(t, eventstore.Backfill(ctx, pg, imp))
	assert.Equal(t, 2, imp.runs)
}
