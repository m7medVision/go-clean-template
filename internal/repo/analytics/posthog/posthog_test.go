package posthog_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/analytics/posthog"
	"github.com/m7medvision/go-clean-template/internal/repo/analytics/sinktest"
	"github.com/stretchr/testify/assert"
)

type fakePostHog struct {
	mu       sync.Mutex
	received []sinktest.Received
	next     int
}

func (f *fakePostHog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.next != 0 {
		w.WriteHeader(f.next)
		f.next = 0

		return
	}

	var body struct {
		APIKey     string `json:"api_key"`
		Event      string `json:"event"`
		DistinctID string `json:"distinct_id"`
		UUID       string `json:"uuid"`
	}
	if r.URL.Path != "/i/v0/e/" || json.NewDecoder(r.Body).Decode(&body) != nil || body.APIKey != "key" {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	f.received = append(f.received, sinktest.Received{Name: body.Event, DistinctID: body.DistinctID, DedupKey: body.UUID})
}

func (f *fakePostHog) Received() []sinktest.Received {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]sinktest.Received(nil), f.received...)
}

func (f *fakePostHog) Answer(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.next = status
}

func TestSink(t *testing.T) {
	t.Parallel()

	sinktest.Run(t, func(t *testing.T) (repo.AnalyticsSink, sinktest.Vendor) {
		t.Helper()

		fake := &fakePostHog{}
		srv := httptest.NewServer(fake)
		t.Cleanup(srv.Close)

		return posthog.New("key", srv.URL), fake
	})
}

func TestUUIDIsStableAndAUUID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, posthog.UUID("task-1-1"), posthog.UUID("task-1-1"))
	assert.Len(t, posthog.UUID("task-1-1"), 36)
}
