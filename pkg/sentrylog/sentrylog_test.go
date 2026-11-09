package sentrylog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/m7medvision/go-clean-template/pkg/sentrylog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

var errRefused = errors.New("conn refused")

type fixture struct {
	transport *sentry.MockTransport
	root      *sentry.Hub
	out       *bytes.Buffer
	l         *slog.Logger
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	transport := &sentry.MockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)

	root := sentry.NewHub(client, sentry.NewScope())
	out := &bytes.Buffer{}
	next := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: slog.LevelInfo})

	return &fixture{transport: transport, root: root, out: out, l: slog.New(sentrylog.New(next, root))}
}

func (f *fixture) fork() context.Context {
	return sentry.SetHubOnContext(context.Background(), f.root.Clone())
}

func TestAnErrorIsOneEventWithTheBreadcrumbsBeforeIt(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	traceID := trace.TraceID{1}
	ctx := trace.ContextWithSpanContext(f.fork(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: trace.SpanID{1},
	}))

	f.l.DebugContext(ctx, "cqrs - message failed", "name", "CreateTask", "kind", "command")
	f.l.ErrorContext(ctx, "restapi - createTask",
		"error", fmt.Errorf("CreateTask - aggregate.Save: %w", errRefused))

	events := f.transport.Events()
	require.Len(t, events, 1)

	ev := events[0]
	assert.Equal(t, "restapi - createTask", ev.Message)
	assert.Equal(t, []string{"restapi - createTask", "CreateTask - aggregate.Save"}, ev.Fingerprint)
	assert.Equal(t, traceID.String(), ev.Tags["trace_id"])
	require.NotEmpty(t, ev.Exception)
	require.Len(t, ev.Breadcrumbs, 1)
	assert.Equal(t, "cqrs", ev.Breadcrumbs[0].Category)
	assert.Equal(t, sentry.LevelDebug, ev.Breadcrumbs[0].Level)
	assert.Equal(t, map[string]any{"name": "CreateTask", "kind": "command"}, ev.Breadcrumbs[0].Data)

	assert.NotContains(t, f.out.String(), "cqrs - message failed", "LOG_LEVEL still filters stdout")
	assert.Contains(t, f.out.String(), "restapi - createTask")
}

func TestRecordsBelowErrorAreNeverSentAlone(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := f.fork()

	f.l.DebugContext(ctx, "a")
	f.l.InfoContext(ctx, "b")
	f.l.WarnContext(ctx, "c")

	assert.Empty(t, f.transport.Events())
}

func TestOnlyAllowedAttributesLeaveTheProcess(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := f.fork()
	l := f.l.With("client", "10.0.0.1")

	l.InfoContext(ctx, "http - request", "uri", "/v1/tasks?q=secret", "method", "GET", "email", "a@b.c")
	l.ErrorContext(ctx, "http panic", "uri", "/v1/tasks?q=secret", "status", "500", "error", errRefused)

	ev := f.transport.Events()[0]
	assert.Equal(t, sentry.Context{"status": "500"}, ev.Contexts["log"])
	assert.Equal(t, map[string]any{"method": "GET"}, ev.Breadcrumbs[0].Data)
}

func TestConcurrentRequestsKeepTheirOwnBreadcrumbs(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	var wg sync.WaitGroup

	for _, name := range []string{"one", "two"} {
		wg.Go(func() {
			ctx := f.fork()
			for range 5 {
				f.l.InfoContext(ctx, "step", "name", name)
			}

			f.l.ErrorContext(ctx, "failed "+name, "error", errRefused)
		})
	}

	wg.Wait()

	events := f.transport.Events()
	require.Len(t, events, 2)

	for _, ev := range events {
		require.Len(t, ev.Breadcrumbs, 5)

		for _, b := range ev.Breadcrumbs {
			assert.Equal(t, fmt.Sprint("failed ", b.Data["name"]), ev.Message)
		}
	}
}

func TestAContextWithoutAHubAddsNoBreadcrumb(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	f.l.Info("startup")
	f.l.Error("app - stop tracing", "error", errRefused)

	events := f.transport.Events()
	require.Len(t, events, 1)
	assert.Empty(t, events[0].Breadcrumbs)
}

func TestTwoOperationsThroughOneCallSiteAreTwoGroups(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := f.fork()

	for _, op := range []string{"CreateTask - aggregate.Save", "GetTask - h.Tasks.Get"} {
		f.l.ErrorContext(ctx, "restapi - op", "error", fmt.Errorf("%s: %w", op, errRefused))
	}

	events := f.transport.Events()
	require.Len(t, events, 2)
	assert.NotEqual(t, events[0].Fingerprint, events[1].Fingerprint)
}

func TestForkGivesEachContextItsOwnHub(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	parent := f.fork()

	a, b := sentrylog.Fork(parent), sentrylog.Fork(parent)
	sentrylog.SetUser(a, "u1")

	assert.NotSame(t, sentry.GetHubFromContext(a), sentry.GetHubFromContext(b))
	f.l.ErrorContext(a, "x", "error", errRefused)
	f.l.ErrorContext(b, "y", "error", errRefused)

	events := f.transport.Events()
	require.Len(t, events, 2)
	assert.Equal(t, "u1", events[0].User.ID)
	assert.Empty(t, events[1].User.ID)
}
