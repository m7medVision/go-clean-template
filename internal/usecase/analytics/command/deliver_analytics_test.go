package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/analytics/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errVendor = errors.New("vendor down")

type fakeSink struct {
	err  error
	sent []*analytics.Event
}

func (*fakeSink) Name() string { return "fake" }

func (s *fakeSink) Send(_ context.Context, e *analytics.Event) error {
	s.sent = append(s.sent, e)

	return s.err
}

func TestDeliverAnalytics(t *testing.T) {
	t.Parallel()

	e := &analytics.Event{ID: "task-t1-1", Properties: analytics.TaskCreated{}}

	tests := []struct {
		name    string
		sink    string
		sinkErr error
		wantErr error
		sent    int
	}{
		{name: "sends to the named sink", sink: "fake", sent: 1},
		{name: "an unknown sink", sink: "nope", wantErr: analytics.ErrUnknownSink},
		{name: "the sink's failure", sink: "fake", sinkErr: errVendor, wantErr: errVendor, sent: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sink := &fakeSink{err: tt.sinkErr}
			h := command.DeliverAnalyticsHandler{Sinks: map[string]repo.AnalyticsSink{"fake": sink}}

			_, err := h.Handle(t.Context(), command.DeliverAnalytics{Sink: tt.sink, Event: e})

			require.ErrorIs(t, err, tt.wantErr)
			assert.Len(t, sink.sent, tt.sent)
		})
	}
}
