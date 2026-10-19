package mail

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/usecase/mail"

// tracedUseCase wraps a Mail usecase with OpenTelemetry spans, closing the gap between the worker's
// handler and the renderer span; the SMTP send itself is untraced and shows as this span's time.
type tracedUseCase struct {
	next usecase.Mail
}

// newTraced wraps a Mail usecase with tracing spans.
func newTraced(next usecase.Mail) usecase.Mail {
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

// Deliver records the template, not the recipient: an address is personal data.
func (t *tracedUseCase) Deliver(ctx context.Context, email entity.Email) error {
	ctx, span := startSpan(ctx, "MailUseCase.Deliver", attribute.String("email.template", email.Template()))

	err := t.next.Deliver(ctx, email)
	endSpan(span, err)

	return err
}
