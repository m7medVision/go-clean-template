package app

import (
	"log/slog"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcuser "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/user"
	restuser "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/user"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/usecase/user"
	userevent "github.com/m7medvision/go-clean-template/internal/usecase/user/event"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"google.golang.org/grpc"
)

// userModule is the user domain: sign-up and password sign-in, its importer and projector, the
// welcome email that follows a sign-up, and its REST and gRPC operations.
func userModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			user.Register(b.Commands, b.Queries, user.Deps{
				Store: d.store, Users: persistUserRepo.NewReadModel(d.pg), Sessions: session.New(d.tokens),
			})

			return nil
		},
		importer: func(d *deps) eventstore.Importer { return persistUserRepo.NewImporter(d.pg) },
		rest:     restuser.NewRoutes,
		grpc: func(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger) {
			pb.RegisterAuthServiceServer(s, grpcuser.New(b, l))
		},
		grpcPublic: grpcuser.PublicMethods(),
		projector:  func(pg *postgres.Postgres) projection.Projector { return persistUserRepo.NewProjector(pg) },
		eventHandlers: func(d *deps) []repo.Subscriber {
			loginURL := d.cfg.App.BaseURL + "/login"

			return []repo.Subscriber{userevent.SendWelcomeEmail{Notifier: notify.New(queue.New(d.pg), loginURL)}}
		},
	}
}
