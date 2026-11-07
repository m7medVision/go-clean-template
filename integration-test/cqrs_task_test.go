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

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

// The web client lists its tasks straight after every write: the write must not answer before the
// read model has it.
func TestHTTPTaskWritesAreReadableAtOnce(t *testing.T) {
	token := registerAndLogin(t)

	created := httpCreateTask(t, token, "read me", "at once")
	listed := httpGetJSON[taskListResponse](t, token, "/tasks")
	assert.Contains(t, taskIDs(listed.Tasks), created.ID)

	require.Equal(t, http.StatusOK, httpUpdateTask(t, token, created.ID, "read me again"))
	assert.Equal(t, "read me again", httpGetJSON[taskResponse](t, token, "/tasks/"+created.ID).Title)
}

func TestGRPCTaskWritesAreReadableAtOnce(t *testing.T) {
	token := registerAndLoginGRPC(t)

	conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	client := protov1.NewTaskServiceClient(conn)
	ctx := grpcAuthCtx(t, token)

	created, err := client.CreateTask(ctx, &protov1.CreateTaskRequest{Title: "read me", Description: "at once"})
	require.NoError(t, err)

	listed, err := client.ListTasks(ctx, &protov1.ListTasksRequest{Limit: 10})
	require.NoError(t, err)

	ids := make([]string, 0, len(listed.GetTasks()))
	for _, task := range listed.GetTasks() {
		ids = append(ids, task.GetId())
	}

	assert.Contains(t, ids, created.GetId())
}

func TestGraphQLTaskWritesAreReadableAtOnce(t *testing.T) {
	token := registerAndLogin(t)

	created := graphqlDo(t, token, `mutation { createTask(input: {title: "read me"}) { id } }`, nil)
	require.Empty(t, created.Errors)

	read := graphqlDo(t, token, `query($id: ID!) { task(id: $id) { id title } }`,
		map[string]any{"id": created.Data.CreateTask.ID})
	require.Empty(t, read.Errors)
	assert.Equal(t, "read me", read.Data.Task.Title)
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
	last, err := queries.New(pg.Executor(t.Context())).LastPosition(t.Context())
	require.NoError(t, err)

	require.NoError(t, projection.Rebuild(t.Context(), pg, "task", persistTaskRepo.NewProjector(pg)))

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
