package user

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"

// tracedRepo wraps the user read model with OpenTelemetry spans.
type tracedRepo struct {
	next repo.UserReadModel
}

func newTraced(next repo.UserReadModel) repo.UserReadModel {
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

func (r *tracedRepo) ByID(ctx context.Context, id string) (entity.User, error) {
	ctx, span := startSpan(ctx, "UserReadModel.ByID", attribute.String("user.id", id))

	result, err := r.next.ByID(ctx, id)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) ByEmail(ctx context.Context, email string) (entity.User, error) {
	ctx, span := startSpan(ctx, "UserReadModel.ByEmail", attribute.String("user.email", email))

	result, err := r.next.ByEmail(ctx, email)
	endSpan(span, err)

	return result, err
}
