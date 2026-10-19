package exchange

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/persistent/exchange"

// tracedRepo wraps an ExchangeRepo with OpenTelemetry spans on top of the
// low-level pgx query spans, giving a semantic "ExchangeRepo.<Method>" view.
type tracedRepo struct {
	next repo.ExchangeRepo
}

func newTraced(next repo.ExchangeRepo) repo.ExchangeRepo {
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

func (r *tracedRepo) Store(ctx context.Context, userID string, c *entity.Conversion) error {
	ctx, span := startSpan(
		ctx, "ExchangeRepo.Store",
		attribute.String("user.id", userID),
		attribute.String("exchange.from", c.From),
		attribute.String("exchange.to", c.To),
	)

	err := r.next.Store(ctx, userID, c)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) GetHistory(
	ctx context.Context, userID string, limit, offset int64,
) ([]entity.Conversion, int, error) {
	ctx, span := startSpan(
		ctx, "ExchangeRepo.GetHistory",
		attribute.String("user.id", userID),
		attribute.Int64("exchange.limit", limit),
		attribute.Int64("exchange.offset", offset),
	)

	history, total, err := r.next.GetHistory(ctx, userID, limit, offset)
	endSpan(span, err)

	return history, total, err
}
