package passkey

import (
	"context"
	"encoding/json"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"

// tracedRepo wraps a PasskeyRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.PasskeyRepo
}

func newTraced(next repo.PasskeyRepo) repo.PasskeyRepo {
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

func (r *tracedRepo) Store(ctx context.Context, passkey *entity.Passkey) error {
	ctx, span := startSpan(ctx, "PasskeyRepo.Store",
		attribute.String("passkey.id", passkey.ID.String()),
		attribute.String("user.id", passkey.UserID),
	)

	err := r.next.Store(ctx, passkey)
	endSpan(span, err)

	return err
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

func (r *tracedRepo) UpdateUsage(ctx context.Context, id entity.PasskeyID, credential json.RawMessage, usedAt time.Time) error {
	ctx, span := startSpan(ctx, "PasskeyRepo.UpdateUsage", attribute.String("passkey.id", id.String()))

	err := r.next.UpdateUsage(ctx, id, credential, usedAt)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) Rename(ctx context.Context, userID string, id entity.PasskeyID, name string) (entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyRepo.Rename",
		attribute.String("passkey.id", id.String()),
		attribute.String("user.id", userID),
	)

	result, err := r.next.Rename(ctx, userID, id, name)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) Delete(ctx context.Context, userID string, id entity.PasskeyID) error {
	ctx, span := startSpan(ctx, "PasskeyRepo.Delete",
		attribute.String("passkey.id", id.String()),
		attribute.String("user.id", userID),
	)

	err := r.next.Delete(ctx, userID, id)
	endSpan(span, err)

	return err
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
