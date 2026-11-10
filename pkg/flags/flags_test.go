package flags_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/flags"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newClient(t *testing.T, file string) (*flags.Client, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer

	c, err := flags.New(t.Context(), file, slog.New(slog.NewTextHandler(&logs, nil)))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Close(context.WithoutCancel(t.Context()))) })

	return c, &logs
}

func TestNoFileAnswersDefaults(t *testing.T) {
	t.Parallel()

	c, logs := newClient(t, "")

	assert.True(t, c.Bool(t.Context(), "any", true, "user-1"))
	assert.False(t, c.Bool(t.Context(), "any", false, "user-1"))
	assert.Empty(t, logs.String())
}

func TestFileAnswersItsValues(t *testing.T) {
	t.Parallel()

	c, _ := newClient(t, "testdata/flags.yaml")

	assert.True(t, c.Bool(t.Context(), "on-flag", false, "user-1"))
	assert.False(t, c.Bool(t.Context(), "off-flag", true, "user-1"))
}

func TestFailedEvaluationAnswersDefaultAndWarnsOnce(t *testing.T) {
	t.Parallel()

	c, logs := newClient(t, "testdata/flags.yaml")

	assert.True(t, c.Bool(t.Context(), "missing", true, "user-1"))
	assert.Equal(t, 1, strings.Count(logs.String(), "flag evaluation failed, default used"))
	assert.Contains(t, logs.String(), "level=WARN")
	assert.Contains(t, logs.String(), "flag=missing")
}

func TestEvaluationAddsEventToActiveSpan(t *testing.T) {
	t.Parallel()

	rec := tracetest.NewSpanRecorder()
	ctx, span := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)).Tracer("test").Start(t.Context(), "message")

	c, err := flags.NewWithProvider(t.Context(), memprovider.NewInMemoryProvider(map[string]memprovider.InMemoryFlag{
		"on-flag": {Key: "on-flag", State: memprovider.Enabled, DefaultVariant: "on", Variants: map[string]any{"on": true}},
	}), slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	assert.True(t, c.Bool(ctx, "on-flag", false, "user-1"))
	span.End()

	require.Len(t, rec.Ended(), 1)
	events := rec.Ended()[0].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "feature_flag.evaluation", events[0].Name)
}
