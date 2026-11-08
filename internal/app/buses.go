package app

import (
	"log/slog"
	"slices"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
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
// trace, log, metrics and stamp on both, then the extra steps the binary adds (the app's Limit);
// then, for commands, wait for the projections, retry on a concurrent write; then authorize and
// validate on both, right before the handler.
// A write waits consistencyTimeout at most for the projectors of what it wrote to pass it. The
// metrics step registers its collectors on metrics, so each binary builds its buses once.
func newBuses(
	pg *postgres.Postgres, projectors []projection.Projector, consistencyTimeout time.Duration,
	metrics prometheus.Registerer, l *slog.Logger, extra ...cqrs.Step,
) cqrs.Buses {
	outer := slices.Concat([]cqrs.Step{
		cqrs.Trace(otel.GetTracerProvider()),
		cqrs.Log(l),
		cqrs.Metrics(metrics),
		behavior.Stamp(),
	}, extra)
	inner := []cqrs.Step{behavior.Authorize(), behavior.Validate()}

	cmds := cqrs.NewCommands(slices.Concat(outer, []cqrs.Step{
		behavior.Wait(projection.NewCheckpoints(pg), projectorsByEvent(projectors), consistencyTimeout, _waitTick),
		behavior.Retry(_commandAttempts),
	}, inner)...)
	qs := cqrs.NewQueries(slices.Concat(outer, inner)...)

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

func newCodec() *eventcodec.Codec {
	return eventcodec.New(entity.EventTypes(), entity.Upcasts()...)
}

func newEventStore(pg *postgres.Postgres, snapshotEvery int) repo.EventStore {
	return eventstore.New(pg, newCodec(), snapshotEvery)
}

// projectorsByEvent names, for each event type, the projectors that handle it: what a write of that
// type waits for.
func projectorsByEvent(projectors []projection.Projector) map[string][]string {
	byEvent := map[string][]string{}

	for _, p := range projectors {
		for _, e := range p.Handles() {
			byEvent[e.EventType()] = append(byEvent[e.EventType()], p.Name())
		}
	}

	return byEvent
}
