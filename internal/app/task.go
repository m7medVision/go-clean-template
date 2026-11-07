package app

import (
	"log/slog"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpctask "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/task"
	resttask "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/task"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc"
)

// taskModule is the task domain: its handlers over the task read model, its importer and projector,
// and its REST and gRPC operations.
func taskModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			task.Register(b.Commands, b.Queries, task.Deps{Store: d.store, Tasks: persistTaskRepo.NewReadModel(d.pg)})

			return nil
		},
		importer: func(d *deps) eventstore.Importer { return persistTaskRepo.NewImporter(d.pg) },
		rest:     resttask.NewRoutes,
		grpc: func(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger) {
			pb.RegisterTaskServiceServer(s, grpctask.New(b, l))
		},
		projector: persistTaskRepo.NewProjector,
	}
}
