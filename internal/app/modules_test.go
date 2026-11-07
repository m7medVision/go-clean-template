package app

import (
	"log/slog"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mailcommand "github.com/m7medvision/go-clean-template/internal/usecase/mail/command"
	oidcquery "github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownEmail is an email no template renders, so delivering it fails before any SMTP.
type unknownEmail struct{}

func (unknownEmail) Template() string  { return "unknown" }
func (unknownEmail) Recipient() string { return "john@example.com" }

// testDeps builds the modules from valid settings, on a pool that is never connected: building a
// part opens no connection.
func testDeps(tokens *jwt.Manager) *deps {
	pg := &postgres.Postgres{}

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
// those rows.
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

	for _, imp := range importers(d) {
		domains = append(domains, imp.Domain())
	}

	assert.ElementsMatch(t, []string{"task", "user", "exchange", "passkey", "send-welcome-email"}, subscribers)
	assert.ElementsMatch(t, []string{"task", "user", "exchange", "passkey"}, domains)
}

// Every event type reaches a read model, and no two subscribers share a checkpoint.
func TestTheModulesSubscribersRunTogether(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)

	_, err := newRunner(d)
	require.NoError(t, err)
}
