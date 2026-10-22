package passkey

import (
	"context"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/usecase/passkey"

// tracedUseCase wraps a Passkey usecase with OpenTelemetry spans, closing the
// gap between transport spans and repository spans.
type tracedUseCase struct {
	next usecase.Passkey
}

// newTraced wraps a Passkey usecase with tracing spans.
func newTraced(next usecase.Passkey) usecase.Passkey {
	return &tracedUseCase{next: next}
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

func (u *tracedUseCase) BeginRegistration(
	ctx context.Context, userID string, authTime time.Time,
) (options []byte, token string, err error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.BeginRegistration", attribute.String("user.id", userID))

	options, token, err = u.next.BeginRegistration(ctx, userID, authTime)
	endSpan(span, err)

	return options, token, err
}

func (u *tracedUseCase) FinishRegistration(ctx context.Context, userID, token, name string, body []byte) (entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.FinishRegistration", attribute.String("user.id", userID))

	result, err := u.next.FinishRegistration(ctx, userID, token, name, body)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) List(ctx context.Context, userID string) ([]entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.List", attribute.String("user.id", userID))

	result, err := u.next.List(ctx, userID)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Rename(ctx context.Context, userID, id, name string) (entity.Passkey, error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.Rename",
		attribute.String("user.id", userID),
		attribute.String("passkey.id", id),
	)

	result, err := u.next.Rename(ctx, userID, id, name)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Delete(ctx context.Context, userID, id string) error {
	ctx, span := startSpan(ctx, "PasskeyUseCase.Delete",
		attribute.String("user.id", userID),
		attribute.String("passkey.id", id),
	)

	err := u.next.Delete(ctx, userID, id)
	endSpan(span, err)

	return err
}

func (u *tracedUseCase) BeginLogin(ctx context.Context) (options []byte, token string, err error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.BeginLogin")

	options, token, err = u.next.BeginLogin(ctx)
	endSpan(span, err)

	return options, token, err
}

func (u *tracedUseCase) FinishLogin(ctx context.Context, token string, body []byte) (string, error) {
	ctx, span := startSpan(ctx, "PasskeyUseCase.FinishLogin")

	result, err := u.next.FinishLogin(ctx, token, body)
	endSpan(span, err)

	return result, err
}
