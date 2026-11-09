package v1_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/usecase/analytics/command"
	"github.com/m7medvision/go-clean-template/pkg/broker"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errVendorDown = errors.New("vendor down")

type fakeDelivery struct {
	err   error
	sinks []string
}

func (f *fakeDelivery) commands() *cqrs.Commands {
	cmds := cqrs.NewCommands()
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.DeliverAnalytics, struct{}](
		func(_ context.Context, c command.DeliverAnalytics) (struct{}, error) {
			f.sinks = append(f.sinks, c.Sink)

			return struct{}{}, f.err
		}))

	return cmds
}

type fakeRouter struct{ topics []string }

func (r *fakeRouter) Handle(topic string, _ broker.HandlerFunc) { r.topics = append(r.topics, topic) }

func analyticsBody(t *testing.T) []byte {
	t.Helper()

	body, err := analytics.Encode(&analytics.Event{ID: "task-t1-1", DistinctID: "u1", Properties: analytics.TaskCreated{}})
	require.NoError(t, err)

	return body
}

func TestNewAnalyticsRoutesRegistersATopicPerSink(t *testing.T) {
	t.Parallel()

	r := &fakeRouter{}
	v1.NewAnalyticsRoutes(r, cqrs.NewCommands(), []string{"posthog", "other"}, slog.New(slog.DiscardHandler))

	assert.Equal(t, []string{"analytics-posthog", "analytics-other"}, r.topics)
}

func TestAnalyticsController_Handle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          func(t *testing.T) []byte
		deliverErr    error
		wantErr       bool
		wantPermanent bool
		wantSent      int
	}{
		{name: "delivers to the controller's sink", body: analyticsBody, wantSent: 1},
		{
			name:          "a body that does not decode is poison",
			body:          func(*testing.T) []byte { return []byte(`{"name":"nope"}`) },
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "an event the vendor refuses is poison",
			body:          analyticsBody,
			deliverErr:    fmt.Errorf("send: %w", analytics.ErrRejected),
			wantErr:       true,
			wantPermanent: true,
			wantSent:      1,
		},
		{
			name:          "an unknown sink is poison",
			body:          analyticsBody,
			deliverErr:    analytics.ErrUnknownSink,
			wantErr:       true,
			wantPermanent: true,
			wantSent:      1,
		},
		{name: "a vendor that is down is retried", body: analyticsBody, deliverErr: errVendorDown, wantErr: true, wantSent: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := &fakeDelivery{err: tc.deliverErr}
			err := v1.NewAnalyticsController(d.commands(), "posthog", slog.New(slog.DiscardHandler)).Handle(t.Context(), tc.body(t))

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantPermanent, broker.IsPermanent(err))
			assert.Len(t, d.sinks, tc.wantSent)

			for _, s := range d.sinks {
				assert.Equal(t, "posthog", s)
			}
		})
	}
}
