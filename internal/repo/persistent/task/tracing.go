package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"

// tracedRepo wraps the task read model with OpenTelemetry spans on top of the
// low-level pgx query spans, giving a semantic "TaskReadModel.<Method>" view.
type tracedRepo struct {
	next repo.TaskReadModel
}

func newTraced(next repo.TaskReadModel) repo.TaskReadModel {
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

func (r *tracedRepo) Get(ctx context.Context, userID, taskID string) (entity.Task, error) {
	ctx, span := startSpan(
		ctx, "TaskReadModel.GetByID",
		attribute.String("user.id", userID),
		attribute.String("task.id", taskID),
	)

	result, err := r.next.Get(ctx, userID, taskID)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) List(ctx context.Context, userID string, filter repo.TaskFilter) ([]entity.Task, int, error) {
	attrs := []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.Int64("task.limit", filter.Limit),
		attribute.Int64("task.offset", filter.Offset),
	}
	if filter.Status != nil {
		attrs = append(attrs, attribute.String("task.status", string(*filter.Status)))
	}

	ctx, span := startSpan(ctx, "TaskReadModel.List", attrs...)

	tasks, total, err := r.next.List(ctx, userID, filter)
	endSpan(span, err)

	return tasks, total, err
}

func (r *tracedRepo) ListAll(ctx context.Context, filter repo.TaskFilter) ([]entity.Task, int, error) {
	attrs := []attribute.KeyValue{
		attribute.Int64("task.limit", filter.Limit),
		attribute.Int64("task.offset", filter.Offset),
	}
	if filter.Status != nil {
		attrs = append(attrs, attribute.String("task.status", string(*filter.Status)))
	}

	ctx, span := startSpan(ctx, "TaskReadModel.ListAll", attrs...)

	tasks, total, err := r.next.ListAll(ctx, filter)
	endSpan(span, err)

	return tasks, total, err
}
