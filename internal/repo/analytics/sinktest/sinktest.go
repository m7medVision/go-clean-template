// Package sinktest is the contract every analytics sink passes, each against a fake of its vendor.
package sinktest

import (
	"net/http"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Received is one event as the vendor's fake read it.
type Received struct {
	Name       string
	DistinctID string
	DedupKey   string
}

// Vendor is the fake a sink sends to.
type Vendor interface {
	Received() []Received
	// Answer makes the next request answer status.
	Answer(status int)
}

// New returns a sink sending to a fresh fake of its vendor.
type New func(t *testing.T) (repo.AnalyticsSink, Vendor)

func Run(t *testing.T, newSink New) {
	t.Helper()

	t.Run("sends every analytics event once", func(t *testing.T) {
		t.Parallel()

		sink, vendor := newSink(t)

		for _, p := range analytics.Events() {
			require.NoError(t, sink.Send(t.Context(), event("e-"+p.Name(), p)))
		}

		got := vendor.Received()
		require.Len(t, got, len(analytics.Events()))

		for i, p := range analytics.Events() {
			assert.Equal(t, "u1", got[i].DistinctID)
			assert.NotEmpty(t, got[i].Name, p.Name())
		}
	})

	t.Run("the same ID gives the same dedup key", func(t *testing.T) {
		t.Parallel()

		sink, vendor := newSink(t)

		ids := []string{"same", "same", "other"}
		for _, id := range ids {
			require.NoError(t, sink.Send(t.Context(), event(id, analytics.TaskCreated{})))
		}

		got := vendor.Received()
		require.Len(t, got, len(ids))
		assert.Equal(t, got[0].DedupKey, got[1].DedupKey)
		assert.NotEqual(t, got[0].DedupKey, got[2].DedupKey)
	})

	t.Run("a rejected event is ErrRejected, a failing vendor is not", func(t *testing.T) {
		t.Parallel()

		sink, vendor := newSink(t)

		vendor.Answer(http.StatusBadRequest)
		require.ErrorIs(t, sink.Send(t.Context(), event("a", analytics.TaskCreated{})), analytics.ErrRejected)

		vendor.Answer(http.StatusServiceUnavailable)

		err := sink.Send(t.Context(), event("b", analytics.TaskCreated{}))
		require.Error(t, err)
		assert.NotErrorIs(t, err, analytics.ErrRejected)
	})
}

func event(id string, p analytics.Properties) *analytics.Event {
	return &analytics.Event{ID: id, DistinctID: "u1", At: time.Unix(1, 0).UTC(), Properties: p}
}
