package exchange

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/usecase/exchange"

// tracedUseCase wraps an Exchange usecase with OpenTelemetry spans, closing the
// gap between transport spans (HTTP/gRPC) and repository spans.
type tracedUseCase struct {
	next usecase.Exchange
}

// newTraced wraps an Exchange usecase with tracing spans.
func newTraced(next usecase.Exchange) usecase.Exchange {
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

func (t *tracedUseCase) Convert(
	ctx context.Context, userID, from, to string, amount decimal.Decimal,
) (entity.Conversion, error) {
	ctx, span := startSpan(
		ctx, "ExchangeUseCase.Convert",
		attribute.String("user.id", userID),
		attribute.String("exchange.from", from),
		attribute.String("exchange.to", to),
	)

	result, err := t.next.Convert(ctx, userID, from, to, amount)
	endSpan(span, err)

	return result, err
}

func (t *tracedUseCase) History(ctx context.Context, userID string, limit, offset int) (entity.ExchangeHistory, error) {
	ctx, span := startSpan(
		ctx, "ExchangeUseCase.History",
		attribute.String("user.id", userID),
		attribute.Int("exchange.limit", limit),
		attribute.Int("exchange.offset", offset),
	)

	result, err := t.next.History(ctx, userID, limit, offset)
	endSpan(span, err)

	return result, err
}
