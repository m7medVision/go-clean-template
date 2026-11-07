package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	concurrentUpdates = 8
	rebuildWait       = 30 * time.Second
	rebuildTick       = 100 * time.Millisecond
)

type taskListResponse struct {
	Tasks []taskResponse `json:"tasks"`
	Total int            `json:"total"`
}

func httpGetJSON[T any](t *testing.T, token, path string) T {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+path, nil, token)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, path)

	return parseJSON[T](t, resp)
}

// httpUpdateTask sends PUT /v1/tasks/{id} and returns its status code.
func httpUpdateTask(t *testing.T, token, id, title string) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	body := fmt.Sprintf(`{"title":%q,"description":""}`, title)

	resp, err := doAuthenticatedRequest(ctx, http.MethodPut, basePathV1+"/tasks/"+id, bytes.NewBufferString(body), token)
	if !assert.NoError(t, err) {
		return 0
	}

	defer resp.Body.Close()

	return resp.StatusCode
}

func taskIDs(tasks []taskResponse) []string {
	ids := make([]string, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
	}

	return ids
}

// Concurrent updates of one task conflict on its stream; the command bus retries them, and any
// that still lose answer 409. Every update that answered 200 is in the stream: none is lost.
func TestConcurrentTaskUpdatesLoseNoWrite(t *testing.T) {
	token := registerAndLogin(t)
	created := httpCreateTask(t, token, "contended", "")

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		saved  []string
		titles = make([]string, concurrentUpdates)
	)

	for i := range concurrentUpdates {
		titles[i] = fmt.Sprintf("update %d", i)

		wg.Go(func() {
			code := httpUpdateTask(t, token, created.ID, titles[i])
			assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, code)

			if code == http.StatusOK {
				mu.Lock()
				defer mu.Unlock()

				saved = append(saved, titles[i])
			}
		})
	}

	wg.Wait()
	require.NotEmpty(t, saved)

	var updates int

	pg := newPostgres(t)
	err := pg.Executor(t.Context()).QueryRow(t.Context(), "SELECT count(*) FROM events WHERE stream_id = $1 AND type = $2",
		entity.TaskStreamID(created.ID), entity.TaskUpdated{}.EventType()).Scan(&updates)
	require.NoError(t, err)

	assert.Equal(t, len(saved), updates, "every update that answered 200 is in the stream")
	assert.True(t, slices.Contains(saved, httpGetJSON[taskResponse](t, token, "/tasks/"+created.ID).Title))
}

// Rebuilding the task projection empties the tasks table and replays every event into it: once the
// worker has caught up, every read is what it was.
func TestRebuildingTheTaskProjectionRestoresItsReads(t *testing.T) {
	token := registerAndLogin(t)
	httpCreateTask(t, token, "first", "kept")
	second := httpCreateTask(t, token, "second", "moved")

	resp, err := httpTransitionTask(t, token, second.ID, statusInProgress)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	before := httpGetJSON[taskListResponse](t, token, "/tasks")

	pg := newPostgres(t)
	last := lastPosition(t, pg)

	require.NoError(t, projection.Rebuild(t.Context(), pg, "task", []projection.Projector{persistTaskRepo.NewProjector(pg)}, nil))

	checkpoints := projection.NewCheckpoints(pg)

	require.Eventually(t, func() bool {
		reached, err := checkpoints.Reached(context.Background(), []string{"task"}, last)

		return err == nil && reached
	}, rebuildWait, rebuildTick, "the worker replays the task projection")

	assert.Equal(t, before, httpGetJSON[taskListResponse](t, token, "/tasks"))
}

func TestMetricsReportTheTaskProjectionsLag(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doWebRequestWithTimeout(ctx, http.MethodGet, httpURL+"/metrics", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `projection_lag_events{projector="task"}`)
}
