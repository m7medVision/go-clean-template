package passkey

import (
	"context"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"

// tracedRepo wraps a PasskeyRepo with OpenTelemetry spans.
type tracedRepo struct {
	next *Repo
}

func newTraced(next *Repo) *tracedRepo {
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

func (r *tracedRepo) ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyRepo.ListByUser", attribute.String("user.id", userID))

	result, err := r.next.ListByUser(ctx, userID)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) GetByID(ctx context.Context, id entity.PasskeyID) (entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyRepo.GetByID", attribute.String("passkey.id", id.String()))

	result, err := r.next.GetByID(ctx, id)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) BurnChallenge(ctx context.Context, hash []byte, expiresAt time.Time) error {
	ctx, span := startSpan(ctx, "PasskeyRepo.BurnChallenge")

	err := r.next.BurnChallenge(ctx, hash, expiresAt)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) DeleteExpiredChallenges(ctx context.Context) error {
	ctx, span := startSpan(ctx, "PasskeyRepo.DeleteExpiredChallenges")

	err := r.next.DeleteExpiredChallenges(ctx)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) Get(ctx context.Context, userID string, id entity.PasskeyID) (entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyReadModel.Get", attribute.String("user.id", userID))

	passkey, err := r.next.Get(ctx, userID, id)
	endSpan(span, err)

	return passkey, err
}
