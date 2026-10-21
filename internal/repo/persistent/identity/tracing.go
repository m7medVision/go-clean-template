package identity

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/identity"

// tracedRepo wraps an IdentityRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.IdentityRepo
}

func newTraced(next repo.IdentityRepo) repo.IdentityRepo {
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

func (r *tracedRepo) UserByIdentity(ctx context.Context, issuer, subject string) (entity.User, error) {
	ctx, span := startSpan(ctx, "IdentityRepo.UserByIdentity", attribute.String("oidc.issuer", issuer))

	result, err := r.next.UserByIdentity(ctx, issuer, subject)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) CreateWithIdentity(ctx context.Context, user *entity.User, issuer, subject string) error {
	ctx, span := startSpan(
		ctx, "IdentityRepo.CreateWithIdentity",
		attribute.String("user.id", user.ID),
		attribute.String("oidc.issuer", issuer),
	)

	err := r.next.CreateWithIdentity(ctx, user, issuer, subject)
	endSpan(span, err)

	return err
}
