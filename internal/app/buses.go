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
// trace, log, metrics and stamp on both; then, for commands, wait for the projections, retry on a
// concurrent write; then authorize and validate on both, right before the handler.
// A write waits consistencyTimeout at most for the projectors to pass it.
func newBuses(pg *postgres.Postgres, projectors []repo.Subscriber, consistencyTimeout time.Duration, l *slog.Logger) (
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
		behavior.Wait(projection.NewCheckpoints(pg), projectors, consistencyTimeout, _waitTick),
		behavior.Retry(_commandAttempts),
	}, inner)...)
	qs := cqrs.NewQueries(slices.Concat(outer, inner)...)

	return cmds, qs
}

// newCodec returns the codec of the domain's events, as entity lists them and their upcasts.
func newCodec() *eventcodec.Codec {
	return eventcodec.New(entity.EventTypes(), entity.Upcasts()...)
}

// newEventStore returns the event store every command handler appends through. It snapshots a
// stream every snapshotEvery events.
func newEventStore(pg *postgres.Postgres, snapshotEvery int) repo.EventStore {
	return eventstore.New(pg, newCodec(), snapshotEvery)
}

// subscribersOf is the projectors as the subscribers the wait step waits on.
func subscribersOf(projectors []projection.Projector) []repo.Subscriber {
	subs := make([]repo.Subscriber, len(projectors))
	for i, p := range projectors {
		subs[i] = p
	}

	return subs
}
