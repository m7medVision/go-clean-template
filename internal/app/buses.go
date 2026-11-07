package app

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
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

var (
	errDuplicateSubscriber = errors.New("two subscribers share a name")
	errReservedSubscriber  = errors.New("a subscriber name may not start with an underscore")
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

// newEventStore returns the event store every command handler appends through.
func newEventStore(cfg *config.Config, pg *postgres.Postgres) repo.EventStore {
	return eventstore.New(pg, cfg.CQRS.SnapshotEvery, entity.EventTypes(), eventstore.Upcasts()...)
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
		Passkeys: persistPasskeyRepo.NewReadModel(pg), Guard: persistPasskeyRepo.NewGuard(pg), Tx: pg, Sessions: sessions,
	})
	oidc.Register(cmds, qs, oidc.Deps{SSO: signIn, Store: store, Sessions: sessions})
	mail.Register(cmds, qs, mailDeps)

	return nil
}

// subscribers lists every projector and event handler the worker runs, in the order it starts them.
func subscribers(cfg *config.Config, pg *postgres.Postgres) []repo.Subscriber {
	return []repo.Subscriber{
		persistTaskRepo.NewProjector(pg),
		persistUserRepo.NewProjector(pg),
		persistExchangeRepo.NewProjector(pg),
		persistPasskeyRepo.NewProjector(pg),
		userevent.SendWelcomeEmail{Notifier: notify.New(queue.New(pg), loginURL(cfg))},
	}
}

// loginURL is the sign-in page the welcome email links to.
func loginURL(cfg *config.Config) string {
	return strings.TrimSuffix(cfg.App.BaseURL, "/") + "/login"
}

// projectorNames lists the subscribers a write waits for: the projectors. Event handlers, such as
// the one that enqueues the welcome email, run in the background without holding the write up.
func projectorNames(subs []repo.Subscriber) []string {
	names := make([]string, 0, len(subs))

	for _, s := range subs {
		if _, ok := s.(projection.Projector); ok {
			names = append(names, s.Name())
		}
	}

	return names
}

// checkSubscribers fails on two subscribers with one name, which would share a checkpoint, and on
// a name starting with an underscore, which is kept for the backfill markers.
func checkSubscribers(subs []repo.Subscriber) error {
	seen := make(map[string]bool, len(subs))

	for _, s := range subs {
		switch {
		case strings.HasPrefix(s.Name(), "_"):
			return fmt.Errorf("%w: %q", errReservedSubscriber, s.Name())
		case seen[s.Name()]:
			return fmt.Errorf("%w: %q", errDuplicateSubscriber, s.Name())
		}

		seen[s.Name()] = true
	}

	return nil
}
