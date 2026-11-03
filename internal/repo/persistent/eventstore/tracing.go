package eventstore

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"

// tracedStore wraps an EventStore with OpenTelemetry spans on top of the low-level pgx query
// spans, giving a semantic "EventStore.<Method>" view.
type tracedStore struct {
	next repo.EventStore
}

func newTraced(next repo.EventStore) repo.EventStore {
	return &tracedStore{next: next}
}

func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(_tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	span.End()
}

func (t *tracedStore) Save(ctx context.Context, a entity.Aggregate, claims ...entity.Claim) (int64, error) {
	ctx, span := startSpan(ctx, "EventStore.Save",
		attribute.String("stream.id", a.StreamID()),
		attribute.Int("stream.version", a.Version()),
		attribute.Int("events.count", len(a.Pending())),
	)

	position, err := t.next.Save(ctx, a, claims...)
	endSpan(span, err)

	return position, err
}

func (t *tracedStore) Load(ctx context.Context, streamID string, a entity.Aggregate) error {
	ctx, span := startSpan(ctx, "EventStore.Load", attribute.String("stream.id", streamID))

	err := t.next.Load(ctx, streamID, a)
	endSpan(span, err)

	return err
}

func (t *tracedStore) ClaimOwner(ctx context.Context, scope, value string) (string, error) {
	ctx, span := startSpan(ctx, "EventStore.ClaimOwner", attribute.String("claim.scope", scope))

	owner, err := t.next.ClaimOwner(ctx, scope, value)
	endSpan(span, err)

	return owner, err
}
