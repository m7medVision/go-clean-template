package app

import (
	"log/slog"

	pb "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcexchange "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/exchange"
	restexchange "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/exchange"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistExchangeRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc"
)

// exchangeModule is the currency exchange domain: conversions at the rates EXCHANGE_SERVICE_URL
// serves, the history's importer and projector, and its REST and gRPC operations.
func exchangeModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			exchange.Register(b.Commands, b.Queries, exchange.Deps{
				Store: d.store, Rates: webapi.New(d.cfg.Exchange.ServiceURL), Exchanges: persistExchangeRepo.NewReadModel(d.pg),
			})

			return nil
		},
		importer: func(d *deps) eventstore.Importer { return persistExchangeRepo.NewImporter(d.pg) },
		rest:     restexchange.NewRoutes,
		grpc: func(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger) {
			pb.RegisterExchangeServiceServer(s, grpcexchange.New(b, l))
		},
		projector: persistExchangeRepo.NewProjector,
	}
}
