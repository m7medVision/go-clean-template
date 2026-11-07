package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistExchangeRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	persistPasskeyRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
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

	marked, err := queries.New(pg.Executor(ctx)).HasMarker(ctx, "_backfill/"+imp.domain)
	require.NoError(t, err)
	assert.True(t, marked)
}

func TestAFailedImportLeavesNoMarkerAndRunsAgain(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	imp := &countingImporter{domain: "_test-" + uuid.NewString(), fail: true}

	require.ErrorIs(t, eventstore.Backfill(ctx, pg, imp), errImport)

	marked, err := queries.New(pg.Executor(ctx)).HasMarker(ctx, "_backfill/"+imp.domain)
	require.NoError(t, err)
	assert.False(t, marked)

	imp.fail = false
	require.NoError(t, eventstore.Backfill(ctx, pg, imp))
	assert.Equal(t, 2, imp.runs)
}

// importAll runs every domain's importer against the live database, as the app's first start on
// this code would, and waits for the projectors to pass what it appended.
func importAll(t *testing.T, pg *postgres.Postgres) {
	t.Helper()

	ctx := context.Background()
	store := eventstore.New(pg, eventcodec.New(entity.EventTypes()), 0)

	for _, imp := range []eventstore.Importer{
		persistTaskRepo.NewImporter(pg, store),
		persistUserRepo.NewImporter(pg, store),
		persistExchangeRepo.NewImporter(pg, store),
		persistPasskeyRepo.NewImporter(pg, store),
	} {
		require.NoError(t, pg.InTx(ctx, imp.Import), imp.Domain())
	}

	last, err := queries.New(pg.Executor(ctx)).LastPosition(ctx)
	require.NoError(t, err)

	checkpoints := projection.NewCheckpoints(pg)

	require.Eventually(t, func() bool {
		reached, err := checkpoints.Reached(ctx, []string{"task", "user", "exchange", "passkey"}, last)

		return err == nil && reached
	}, rebuildWait, rebuildTick, "the projectors pass the imported events")
}

func eventsOf(t *testing.T, pg *postgres.Postgres, stream string) []string {
	t.Helper()

	rows, err := pg.Executor(context.Background()).Query(context.Background(), "SELECT type FROM events WHERE stream_id = $1 ORDER BY stream_version", stream)
	require.NoError(t, err)

	defer rows.Close()

	var types []string

	for rows.Next() {
		var typ string
		require.NoError(t, rows.Scan(&typ))

		types = append(types, typ)
	}

	require.NoError(t, rows.Err())

	return types
}

func claimOwner(t *testing.T, pg *postgres.Postgres, scope, value string) string {
	t.Helper()

	owner, err := queries.New(pg.Executor(context.Background())).ClaimOwner(context.Background(), queries.ClaimOwnerParams{Scope: scope, Value: value})
	require.NoError(t, err)

	return owner
}

// writeLegacyRows writes a user with an identity, a task, a passkey and a conversion straight into
// the tables, as the app did before event sourcing.
func writeLegacyRows(t *testing.T, pg *postgres.Postgres, userID, taskID, name string, credential []byte) {
	t.Helper()

	at := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{
			"INSERT INTO users (id, username, email, password_hash, created_at, updated_at) VALUES ($1, $2, $3, '', $4, $4)",
			[]any{userID, name, name + "@test.com", at},
		},
		{
			"INSERT INTO user_identities (issuer, subject, user_id) VALUES ('https://legacy.example', $1, $2)",
			[]any{name, userID},
		},
		{
			"INSERT INTO tasks (id, user_id, title, description, status, created_at, updated_at) VALUES ($1, $2, 'old', '', 'done', $3, $3)",
			[]any{taskID, userID, at},
		},
		{
			"INSERT INTO webauthn_credentials (id, user_id, name, credential, created_at) VALUES ($1, $2, 'Laptop', '{}', $3)",
			[]any{credential, userID, at},
		},
		{"INSERT INTO exchange_history (user_id, from_currency, to_currency, amount, rate, result, rate_date, created_at) " +
			"VALUES ($1, 'OMR', 'CNY', 100, 17.4, 1740, $2, $3)", []any{userID, at, at}},
	} {
		_, err := pg.Executor(context.Background()).Exec(context.Background(), stmt.sql, stmt.args...)
		require.NoError(t, err, stmt.sql)
	}
}

// Rows written before event sourcing, straight into each domain's tables, become their streams'
// first events, with the claims they hold; the rows themselves are left as they were, and a second
// import adds nothing.
func TestBackfillImportsRowsWrittenBeforeEventSourcing(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)

	userID, taskID, credential := uuid.NewString(), uuid.NewString(), []byte(uuid.NewString())
	name := "legacy-" + userID[:8]

	writeLegacyRows(t, pg, userID, taskID, name, credential)

	var taskBefore string

	require.NoError(t, pg.Executor(ctx).QueryRow(ctx, "SELECT row_to_json(t)::text FROM tasks t WHERE id = $1", taskID).Scan(&taskBefore))

	importAll(t, pg)

	assert.Equal(t, []string{"TaskImported"}, eventsOf(t, pg, entity.TaskStreamID(taskID)))
	assert.Equal(t, []string{"UserImported"}, eventsOf(t, pg, entity.UserStreamID(userID)))
	assert.Equal(t, []string{"PasskeyImported"}, eventsOf(t, pg, entity.PasskeyStreamID(credential)))

	assert.Equal(t, userID, claimOwner(t, pg, entity.ClaimUserEmail, name+"@test.com"))
	assert.Equal(t, userID, claimOwner(t, pg, entity.ClaimUserUsername, name))
	assert.Equal(t, userID, claimOwner(t, pg, entity.ClaimOIDCIdentity, entity.OIDCIdentity("https://legacy.example", name)))

	var conversionID string

	require.NoError(t, pg.Executor(ctx).QueryRow(ctx,
		"SELECT conversion_id::text FROM exchange_history WHERE user_id = $1", userID).Scan(&conversionID))
	assert.Equal(t, []string{"ConversionImported"}, eventsOf(t, pg, entity.ConversionStreamID(conversionID)))

	var taskAfter string

	require.NoError(t, pg.Executor(ctx).QueryRow(ctx, "SELECT row_to_json(t)::text FROM tasks t WHERE id = $1", taskID).Scan(&taskAfter))
	assert.JSONEq(t, taskBefore, taskAfter, "projecting the import leaves the row as it was")

	var users int

	require.NoError(t, pg.Executor(ctx).QueryRow(ctx, "SELECT count(*) FROM users WHERE id = $1", userID).Scan(&users))
	assert.Equal(t, 1, users)

	importAll(t, pg)

	assert.Len(t, eventsOf(t, pg, entity.TaskStreamID(taskID)), 1, "a second import adds nothing")
	assert.Len(t, eventsOf(t, pg, entity.ConversionStreamID(conversionID)), 1)
}
