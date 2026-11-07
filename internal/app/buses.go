package app

import (
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistExchangeRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	persistPasskeyRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/user"
	userevent "github.com/m7medvision/go-clean-template/internal/usecase/user/event"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
)

const (
	// _waitTick is how often a write checks whether its projections have caught up.
	_waitTick = 20 * time.Millisecond
	// _commandAttempts is how many times a command runs before a concurrent write wins for good.
	_commandAttempts = 3
)

// newBuses builds the command and query buses with their pipelines, outermost step first:
// trace, log, metrics and stamp on both; then, for commands, wait for the projections, retry on a
// concurrent write; then authorize and validate on both, right before the handler.
func newBuses(cfg *config.Config, pg *postgres.Postgres, projectors []string, l *slog.Logger) (
	*cqrs.Commands, *cqrs.Queries,
) {
	outer := []cqrs.Step{
		cqrs.Trace(otel.GetTracerProvider()),
		cqrs.Log(l),
		cqrs.Metrics(prometheus.DefaultRegisterer),
		behavior.Stamp(),
	}
	inner := []cqrs.Step{behavior.Authorize(), behavior.Validate()}

	cmds := cqrs.NewCommands(slices.Concat(outer, []cqrs.Step{
		behavior.Wait(projection.NewCheckpoints(pg), projectors, cfg.CQRS.ConsistencyTimeout, _waitTick),
		behavior.Retry(_commandAttempts),
	}, inner)...)
	qs := cqrs.NewQueries(slices.Concat(outer, inner)...)

	return cmds, qs
}

// newCodec returns the codec of the domain's events, as entity lists them and their upcasts.
func newCodec() *eventcodec.Codec {
	return eventcodec.New(entity.EventTypes(), entity.Upcasts()...)
}

// newEventStore returns the event store every command handler appends through.
func newEventStore(cfg *config.Config, pg *postgres.Postgres) repo.EventStore {
	return eventstore.New(pg, newCodec(), cfg.CQRS.SnapshotEvery)
}

// registerHandlers registers every domain's command and query handlers on the buses.
func registerHandlers(
	cfg *config.Config, cmds *cqrs.Commands, qs *cqrs.Queries, store repo.EventStore, pg *postgres.Postgres,
	tokens *jwt.Manager,
) error {
	passkeyCeremony, err := newPasskeyCeremony(cfg)
	if err != nil {
		return err
	}

	signIn, err := newSSO(cfg)
	if err != nil {
		return err
	}

	mailDeps, err := newMailDeps(cfg)
	if err != nil {
		return err
	}

	sessions := session.New(tokens)

	task.Register(cmds, qs, task.Deps{Store: store, Tasks: persistTaskRepo.NewReadModel(pg)})
	user.Register(cmds, qs, user.Deps{Store: store, Users: persistUserRepo.NewReadModel(pg), Sessions: sessions})
	exchange.Register(cmds, qs, exchange.Deps{
		Store: store, Rates: webapi.New(cfg.Exchange.ServiceURL), History: persistExchangeRepo.NewReadModel(pg),
	})
	passkey.Register(cmds, qs, &passkey.Deps{
		Ceremony: passkeyCeremony, Store: store, Users: persistUserRepo.NewReadModel(pg),
		Passkeys: persistPasskeyRepo.NewReadModel(pg), Guard: persistPasskeyRepo.NewGuard(pg), Sessions: sessions,
	})
	oidc.Register(cmds, qs, oidc.Deps{SSO: signIn, Store: store, Sessions: sessions})
	mail.Register(cmds, qs, mailDeps)

	return nil
}

// projectors lists every read model's projector: a write waits for them, and each can be rebuilt.
func projectors(pg *postgres.Postgres) []projection.Projector {
	return []projection.Projector{
		persistTaskRepo.NewProjector(pg),
		persistUserRepo.NewProjector(pg),
		persistExchangeRepo.NewProjector(pg),
		persistPasskeyRepo.NewProjector(pg),
	}
}

// eventHandlers lists the side effects that follow events. Nothing waits for them, and they are
// never rebuilt: replaying one would repeat its side effects.
func eventHandlers(cfg *config.Config, pg *postgres.Postgres) []repo.Subscriber {
	return []repo.Subscriber{
		userevent.SendWelcomeEmail{Notifier: notify.New(queue.New(pg), loginURL(cfg))},
	}
}

// loginURL is the sign-in page the welcome email links to.
func loginURL(cfg *config.Config) string {
	return strings.TrimSuffix(cfg.App.BaseURL, "/") + "/login"
}
