package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/worker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/persistent/queue"

// tracedRepo wraps a QueueRepo with OpenTelemetry spans on top of the
// low-level pgx query spans, giving a semantic "QueueRepo.<Method>" view.
type tracedRepo struct {
	next repo.QueueRepo
}

func newTraced(next repo.QueueRepo) repo.QueueRepo {
	return &tracedRepo{next: next}
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

func (r *tracedRepo) Send(ctx context.Context, queue string, body json.RawMessage) (int64, error) {
	ctx, span := startSpan(ctx, "QueueRepo.Send", attribute.String("queue.name", queue))

	id, err := r.next.Send(ctx, queue, body)
	span.SetAttributes(attribute.Int64("queue.message.id", id))
	endSpan(span, err)

	return id, err
}

func (r *tracedRepo) Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error) {
	ctx, span := startSpan(
		ctx, "QueueRepo.Read",
		attribute.String("queue.name", queue),
		attribute.Int("queue.qty", qty),
	)

	msgs, err := r.next.Read(ctx, queue, vt, qty)
	span.SetAttributes(attribute.Int("queue.read", len(msgs)))
	endSpan(span, err)

	return msgs, err
}

func (r *tracedRepo) Delete(ctx context.Context, queue string, id int64) error {
	ctx, span := startSpan(
		ctx, "QueueRepo.Delete",
		attribute.String("queue.name", queue),
		attribute.Int64("queue.message.id", id),
	)

	err := r.next.Delete(ctx, queue, id)
	endSpan(span, err)

	return err
}
