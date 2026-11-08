package mongo

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/pkg/mongo"

type spans struct {
	mu   sync.Mutex
	open map[int64]trace.Span
}

func newMonitor() *event.CommandMonitor {
	s := &spans{open: make(map[int64]trace.Span)}

	return &event.CommandMonitor{Started: s.started, Succeeded: s.succeeded, Failed: s.failed}
}

func (s *spans) started(ctx context.Context, e *event.CommandStartedEvent) {
	_, span := otel.Tracer(_tracerName).Start(ctx, e.CommandName+" "+e.DatabaseName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemMongoDB,
			semconv.DBNamespace(e.DatabaseName),
			semconv.DBOperationName(e.CommandName),
			attribute.String("server.address", e.ConnectionID),
		))

	s.mu.Lock()
	s.open[e.RequestID] = span
	s.mu.Unlock()
}

func (s *spans) succeeded(_ context.Context, e *event.CommandSucceededEvent) {
	if span := s.take(e.RequestID); span != nil {
		span.End()
	}
}

func (s *spans) failed(_ context.Context, e *event.CommandFailedEvent) {
	if span := s.take(e.RequestID); span != nil {
		span.RecordError(e.Failure)
		span.SetStatus(codes.Error, e.Failure.Error())
		span.End()
	}
}

func (s *spans) take(id int64) trace.Span {
	s.mu.Lock()
	defer s.mu.Unlock()

	span := s.open[id]
	delete(s.open, id)

	return span
}
