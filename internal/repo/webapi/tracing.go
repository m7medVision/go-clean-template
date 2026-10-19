package webapi

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/webapi"

// tracedExchangeWebAPI wraps an ExchangeWebAPI with OpenTelemetry spans
// for the outbound call to the rate provider.
type tracedExchangeWebAPI struct {
	next repo.ExchangeWebAPI
}

func newTraced(next repo.ExchangeWebAPI) repo.ExchangeWebAPI {
	return &tracedExchangeWebAPI{next: next}
}

func (t *tracedExchangeWebAPI) Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error) {
	ctx, span := otel.Tracer(_tracerName).Start(
		ctx, "ExchangeWebAPI.Rate",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("exchange.from", from),
			attribute.String("exchange.to", to),
		),
	)

	rate, date, err := t.next.Rate(ctx, from, to)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	span.End()

	return rate, date, err
}
