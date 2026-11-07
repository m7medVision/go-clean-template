package cqrs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// commandsWith returns a command bus with step whose CreateThing handler fails with err (nil for
// success).
func commandsWith(step cqrs.Step, err error) *cqrs.Commands {
	cmds := cqrs.NewCommands(step)
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[createThing, cqrs.Written](
		func(context.Context, createThing) (cqrs.Written, error) { return cqrs.Written{}, err },
	))

	return cmds
}

func TestTraceOpensOneSpanPerMessage(t *testing.T) {
	t.Parallel()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Trace(tp), nil), createThing{})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "command CreateThing", spans[0].Name)
	assert.Equal(t, codes.Unset, spans[0].Status.Code)
}

func TestTraceMarksAFailedMessage(t *testing.T) {
	t.Parallel()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Trace(tp), errHandler), createThing{})
	require.ErrorIs(t, err, errHandler)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, errHandler.Error(), spans[0].Status.Description)
}

func TestLogRecordsTheMessageAndItsOutcome(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Log(l), nil), createThing{})
	require.NoError(t, err)

	_, err = cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Log(l), errHandler), createThing{})
	require.ErrorIs(t, err, errHandler)

	var records []map[string]any

	dec := json.NewDecoder(&buf)
	for dec.More() {
		var r map[string]any
		require.NoError(t, dec.Decode(&r))

		records = append(records, r)
	}

	require.Len(t, records, 2)

	for _, r := range records {
		assert.Equal(t, "command", r["kind"])
		assert.Equal(t, "CreateThing", r["name"])
		assert.Contains(t, r, "duration")
	}

	assert.NotContains(t, records[0], "error")
	assert.Equal(t, errHandler.Error(), records[1]["error"])
}

func TestMetricsCountsMessagesByOutcome(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Metrics(reg), nil), createThing{})
	require.NoError(t, err)

	_, err = cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Metrics(reg), errHandler), createThing{})
	require.ErrorIs(t, err, errHandler)

	expected := `
# HELP cqrs_messages_total Commands and queries handled, by bus, message and outcome.
# TYPE cqrs_messages_total counter
cqrs_messages_total{kind="command",name="CreateThing",outcome="error"} 1
cqrs_messages_total{kind="command",name="CreateThing",outcome="ok"} 1
`
	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expected), "cqrs_messages_total"))

	series, err := testutil.GatherAndCount(reg, "cqrs_message_duration_seconds")
	require.NoError(t, err)
	assert.Equal(t, 2, series, "one duration series per outcome")
}
