package cqrs

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/m7medvision/go-clean-template/pkg/cqrs"

// Trace opens one span per message, named after its bus and message ("command CreateTask"), and
// marks it failed when the message fails. It sits between the transport's span and the
// repositories' spans.
func Trace(tp trace.TracerProvider) Step {
	tracer := tp.Tracer(_tracerName)

	return func(ctx context.Context, msg Message, next Next) (any, error) {
		ctx, span := tracer.Start(ctx, string(msg.Kind)+" "+msg.Name, trace.WithAttributes(
			attribute.String("cqrs.kind", string(msg.Kind)),
			attribute.String("cqrs.name", msg.Name),
		))
		defer span.End()

		out, err := next(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return out, err
	}
}
