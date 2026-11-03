package cqrs_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

	l := &recordingLogger{}

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Log(l), nil), createThing{})
	require.NoError(t, err)

	_, err = cqrs.SendCommand[cqrs.Written](context.Background(), commandsWith(cqrs.Log(l), errHandler), createThing{})
	require.ErrorIs(t, err, errHandler)

	lines := l.lines()
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "cqrs - command CreateThing - ok in "), lines[0])
	assert.True(t, strings.HasPrefix(lines[1], "cqrs - command CreateThing - error in "), lines[1])
	assert.True(t, strings.HasSuffix(lines[1], ": "+errHandler.Error()), lines[1])
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

// recordingLogger keeps every Debug line.
type recordingLogger struct {
	mu    sync.Mutex
	debug []string
}

func (l *recordingLogger) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.debug...)
}

func (l *recordingLogger) Debug(message any, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.debug = append(l.debug, fmt.Sprintf(fmt.Sprint(message), args...))
}

func (*recordingLogger) Info(string, ...any) {}
func (*recordingLogger) Warn(string, ...any) {}
func (*recordingLogger) Error(any, ...any)   {}
func (*recordingLogger) Fatal(any, ...any)   {}
