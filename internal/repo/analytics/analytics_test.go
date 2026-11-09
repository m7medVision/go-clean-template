package analytics_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	analyticsrepo "github.com/m7medvision/go-clean-template/internal/repo/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type published struct{ topic, id string }

type fakePublisher struct{ got []published }

func (f *fakePublisher) Publish(_ context.Context, topic, id string, _ []byte) error {
	f.got = append(f.got, published{topic, id})

	return nil
}

func TestCapturePublishesOncePerSink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sinks []string
		want  []published
	}{
		{name: "no sinks", sinks: nil, want: nil},
		{name: "two sinks", sinks: []string{"posthog", "other"}, want: []published{
			{"analytics-posthog", "posthog-task-1-1"}, {"analytics-other", "other-task-1-1"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pub := &fakePublisher{}
			err := analyticsrepo.New(pub, tt.sinks).Capture(t.Context(),
				&analytics.Event{ID: "task-1-1", DistinctID: "u1", Properties: analytics.TaskCreated{}})

			require.NoError(t, err)
			assert.Equal(t, tt.want, pub.got)
		})
	}
}
