package oidc

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/usecase/oidc"

// tracedUseCase wraps an OIDC usecase with OpenTelemetry spans, closing the
// gap between transport spans (HTTP/gRPC/GraphQL) and repository spans.
type tracedUseCase struct {
	next usecase.OIDC
}

// newTraced wraps an OIDC usecase with tracing spans.
func newTraced(next usecase.OIDC) usecase.OIDC {
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

func (t *tracedUseCase) Enabled() bool {
	return t.next.Enabled()
}

func (t *tracedUseCase) Begin(ctx context.Context) (authURL, token string, err error) {
	ctx, span := startSpan(ctx, "OIDCUseCase.Begin")

	authURL, token, err = t.next.Begin(ctx)
	endSpan(span, err)

	return authURL, token, err
}

func (t *tracedUseCase) Finish(ctx context.Context, code, state, token string) (string, error) {
	ctx, span := startSpan(ctx, "OIDCUseCase.Finish", attribute.Bool("oidc.code_present", code != ""))

	result, err := t.next.Finish(ctx, code, state, token)
	endSpan(span, err)

	return result, err
}
