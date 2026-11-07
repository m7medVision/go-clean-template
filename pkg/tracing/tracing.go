// Package tracing configures OpenTelemetry tracing with an OTLP/gRPC exporter.
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const _defaultShutdownTimeout = 5 * time.Second

// ShutdownFunc flushes and stops the tracer provider.
type ShutdownFunc func(context.Context) error

// Config is where spans go, and how many: Endpoint is the OTLP/gRPC collector, SampleRate the
// share of new traces kept.
type Config struct {
	Enabled    bool
	Endpoint   string
	Insecure   bool
	SampleRate float64
}

// New configures a global TracerProvider with an OTLP/gRPC exporter and
// W3C trace-context + baggage propagators, for the service named service at version. If tracing is
// disabled, it installs a no-op provider so instrumentation code can run unconditionally.
func New(ctx context.Context, service, version string, cfg Config) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporterOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("tracing - New - otlptracegrpc.New: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(service),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing - New - resource.New: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
	)

	otel.SetTracerProvider(tp)

	shutdown := func(shutdownCtx context.Context) error {
		timeoutCtx, cancel := context.WithTimeout(shutdownCtx, _defaultShutdownTimeout)
		defer cancel()

		if err := tp.Shutdown(timeoutCtx); err != nil {
			return fmt.Errorf("tracing - shutdown - tp.Shutdown: %w", err)
		}

		return nil
	}

	return shutdown, nil
}
