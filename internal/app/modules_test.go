package app

import (
	"log/slog"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/sso"
	mailcommand "github.com/m7medvision/go-clean-template/internal/usecase/mail/command"
	oidcquery "github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// unknownEmail is an email no template renders, so delivering it fails before any SMTP.
type unknownEmail struct{}

func (unknownEmail) Template() string  { return "unknown" }
func (unknownEmail) Recipient() string { return "john@example.com" }

// testDeps builds the modules from valid settings, on a pool that is never connected: building a
// part opens no connection.
func testDeps(tokens *jwt.Manager) *deps {
	pg := &postgres.Postgres{}

	client, err := mgo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		panic(err)
	}

	return &deps{
		cfg: &config.Config{
			App: config.App{BaseURL: "http://localhost"},
			JWT: config.JWT{Secret: "test-secret"},
			WebAuthn: config.WebAuthn{
				RPID: "localhost", RPName: "test", RPOrigins: []string{"http://localhost"},
				CeremonyTimeout: time.Minute, ReauthMaxAge: time.Minute,
			},
			Mail: config.Mail{SMTPHost: "localhost", SMTPPort: 1025, From: "noreply@example.com"},
		},
		pg:     pg,
		store:  newEventStore(pg, 0),
		l:      slog.New(slog.DiscardHandler),
		tokens: tokens,
		mongo:  client.Database("test"),
	}
}

func newTestBuses() cqrs.Buses {
	return cqrs.Buses{Commands: cqrs.NewCommands(), Queries: cqrs.NewQueries()}
}

// The app serves every domain but sends no email: the worker does.
func TestTheAppInstallsEveryHandlerButMailDelivery(t *testing.T) {
	t.Parallel()

	b := newTestBuses()
	require.NoError(t, registerHandlers(testDeps(jwt.New("test-secret", time.Hour)), b))

	enabled, err := cqrs.SendQuery(t.Context(), b.Queries, oidcquery.GetOIDCStatus{})
	require.NoError(t, err)
	assert.False(t, enabled)

	_, err = cqrs.SendCommand(t.Context(), b.Commands, mailcommand.DeliverEmail{Email: unknownEmail{}})
	require.ErrorIs(t, err, cqrs.ErrNoHandler)
}

// The worker installs only what its queues send to, so it builds no sign-in, and needs no JWT.
func TestTheWorkerInstallsOnlyWhatItsQueuesSendTo(t *testing.T) {
	t.Parallel()

	b := newTestBuses()
	w := worker.New(slog.New(slog.DiscardHandler), nil, worker.Settings{})
	require.NoError(t, registerQueues(testDeps(nil), b, w))

	_, err := cqrs.SendCommand(t.Context(), b.Commands, mailcommand.DeliverEmail{Email: unknownEmail{}})
	require.ErrorIs(t, err, entity.ErrUnknownEmail)

	_, err = cqrs.SendQuery(t.Context(), b.Queries, oidcquery.GetOIDCStatus{})
	require.ErrorIs(t, err, cqrs.ErrNoHandler)
}

func TestABadRoleMapStopsTheApp(t *testing.T) {
	t.Parallel()

	d := testDeps(jwt.New("test-secret", time.Hour))
	d.cfg.OIDC.RoleMap = map[string]string{"root": "superuser"}

	require.ErrorIs(t, registerHandlers(d, newTestBuses()), sso.ErrUnknownRole)
}

func TestABadMailSenderStopsTheWorker(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)
	d.cfg.Mail.From = "not an address"

	w := worker.New(slog.New(slog.DiscardHandler), nil, worker.Settings{})
	require.Error(t, registerQueues(d, newTestBuses(), w))
}

// A subscriber's name keys its checkpoint row, and an importer's domain its backfill marker. Renaming
// one orphans the stored row: the subscriber starts again from the first event (an event handler
// repeats its side effects), and the importer runs again. Change this list only with a migration of
// those rows. A new domain adds names, which orphans nothing.
func TestTheStoredNamesStayTheSame(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)

	var subscribers, domains []string

	for _, p := range projectors(d.pg) {
		subscribers = append(subscribers, p.Name())
	}

	for _, h := range eventHandlers(d) {
		subscribers = append(subscribers, h.Name())
	}

	for _, imp := range importers(d.pg) {
		domains = append(domains, imp.Domain())
	}

	assert.Subset(t, subscribers, []string{"task", "user", "exchange", "passkey", "send-welcome-email"})
	assert.Subset(t, domains, []string{"task", "user", "exchange", "passkey"})
}

// Every event type reaches a read model, and no two subscribers share a checkpoint.
func TestTheModulesSubscribersRunTogether(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)

	_, err := newRunner(d)
	require.NoError(t, err)
}

// A write waits for the projector of each event type it wrote: every type has one, its domain's.
func TestAWriteWaitsForItsDomainsProjector(t *testing.T) {
	t.Parallel()

	byEvent := projectorsByEvent(projectors(testDeps(nil).pg))

	assert.Len(t, byEvent, len(entity.EventTypes()))
	assert.Equal(t, []string{"task"}, byEvent[entity.TaskCreated{}.EventType()])
	assert.Equal(t, []string{"passkey"}, byEvent[entity.PasskeyUsed{}.EventType()])
}
