package app

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/pkg/broker"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/flags"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	vk "github.com/valkey-io/valkey-go"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
)

// module is one domain at the composition root: everything either binary builds for it, in one
// file per domain. Each binary calls only the parts it runs; a part the domain does not have is nil.
type module struct {
	// The app runs these.

	// handlers registers the domain's command and query handlers on the app's buses.
	handlers func(d *deps, b cqrs.Buses) error
	// importer turns the rows the domain wrote before it stored events into events.
	importer func(pg *postgres.Postgres) eventstore.Importer
	// rest registers its REST operations, grpc its gRPC services. A v2 registers next to v1.
	rest func(api huma.API, b cqrs.Buses, l *slog.Logger)
	grpc func(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger)
	// grpcPublic are the full names of its gRPC methods a caller reaches without a token. A REST
	// operation says so itself, by declaring no Security.
	grpcPublic []string

	// The worker runs these; the app builds the projectors too, to wait for them after a write.

	// projector keeps the domain's read model.
	projector func(pg *postgres.Postgres) projection.Projector
	// eventHandlers are the side effects that follow its events.
	eventHandlers func(d *deps) []repo.Subscriber
	// queues registers the topics the domain consumes on r, and the handlers their messages send
	// to on the worker's buses.
	queues func(d *deps, b cqrs.Buses, r queueRouter) error
}

type deps struct {
	cfg *config.Config
	pg  *postgres.Postgres
	l   *slog.Logger
	// store is the event store the command handlers write through. Only the app has one: the
	// worker's handlers record no event.
	store repo.EventStore
	// tokens signs and checks the auth token. Only the app has one: the worker issues none.
	tokens *jwt.Manager
	valkey vk.Client
	mongo  *mgo.Database
	// flags evaluates feature flags. Only the app has one: no worker handler checks a flag.
	flags *flags.Client
	// broker publishes to NATS. Only the worker has one: the app never connects to NATS.
	broker notify.Publisher
	// sinks are the analytics vendors configured, and analytics publishes to them; only the worker
	// has them, and analytics is nil when there is none.
	sinks     map[string]repo.AnalyticsSink
	analytics repo.Analytics
}

type queueRouter interface {
	Handle(topic string, h broker.HandlerFunc)
}

// modules lists every domain. GraphQL is the one transport not registered here: gqlgen needs one
// root resolver, so internal/controller/graphql/resolver.go lists the domains it serves.
func modules() []module {
	return []module{
		taskModule(),
		userModule(),
		exchangeModule(),
		passkeyModule(),
		oidcModule(),
		mailModule(),
		analyticsModule(),
	}
}

// projectors lists every read model's projector: a write waits for them, and each can be rebuilt.
func projectors(pg *postgres.Postgres) []projection.Projector {
	var ps []projection.Projector

	for _, m := range modules() {
		if m.projector != nil {
			ps = append(ps, m.projector(pg))
		}
	}

	return ps
}

// eventHandlers lists the side effects that follow events. Nothing waits for them, and they are
// never rebuilt: replaying one would repeat its side effects.
func eventHandlers(d *deps) []repo.Subscriber {
	var hs []repo.Subscriber

	for _, m := range modules() {
		if m.eventHandlers != nil {
			hs = append(hs, m.eventHandlers(d)...)
		}
	}

	return hs
}

// registerHandlers registers every domain's command and query handlers on the app's buses. It
// fails when a handler cannot be built from its settings, such as invalid WebAuthn settings.
func registerHandlers(d *deps, b cqrs.Buses) error {
	for _, m := range modules() {
		if m.handlers == nil {
			continue
		}

		if err := m.handlers(d, b); err != nil {
			return err
		}
	}

	return nil
}

func registerREST(b cqrs.Buses, l *slog.Logger) func(api huma.API) {
	return func(api huma.API) {
		for _, m := range modules() {
			if m.rest != nil {
				m.rest(api, b, l)
			}
		}
	}
}

func registerGRPC(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger) {
	for _, m := range modules() {
		if m.grpc != nil {
			m.grpc(s, b, l)
		}
	}
}

// registerQueues registers every queue the worker consumes on w, and their handlers on b. It fails
// when a handler cannot be built from its settings, such as a broken email template.
func registerQueues(d *deps, b cqrs.Buses, r queueRouter) error {
	for _, m := range modules() {
		if m.queues == nil {
			continue
		}

		if err := m.queues(d, b, r); err != nil {
			return err
		}
	}

	return nil
}
