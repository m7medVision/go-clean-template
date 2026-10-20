package mailtmpl

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"

// tracedRenderer wraps a MailRenderer with OpenTelemetry spans.
type tracedRenderer struct {
	next repo.MailRenderer
}

func newTraced(next repo.MailRenderer) repo.MailRenderer {
	return &tracedRenderer{next: next}
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

func (r *tracedRenderer) Render(ctx context.Context, email entity.Email) (smtp.Message, error) {
	ctx, span := startSpan(ctx, "MailRenderer.Render", attribute.String("email.template", email.Template()))

	msg, err := r.next.Render(ctx, email)
	endSpan(span, err)

	return msg, err
}
