package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	statusTodo       = "todo"
	statusInProgress = "in_progress"
)

type taskResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// httpCreateTask is a helper that creates a task via HTTP and returns the parsed response.
func httpCreateTask(t *testing.T, token, title, description string) taskResponse {
	t.Helper()

	createBody := fmt.Sprintf(`{"title":%q,"description":%q}`, title, description)

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodPost, basePathV1+"/tasks/", bytes.NewBufferString(createBody), token)
	if err != nil {
		t.Fatalf("Create task: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Create task: expected 201, got %d", resp.StatusCode)
	}

	return parseJSON[taskResponse](t, resp)
}

// httpTransitionTask sends a PATCH request to transition a task's status.
func httpTransitionTask(t *testing.T, token, id, status string) (*http.Response, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	body := fmt.Sprintf(`{"status":%q}`, status)

	return doAuthenticatedRequest(ctx, http.MethodPatch, basePathV1+"/tasks/"+id+"/status", bytes.NewBufferString(body), token)
}

// HTTP: the task CRUD flow, every write read back at once: the web client lists its tasks straight
// after every write, so a write must not answer before the read model has it.
func TestHTTPTaskV1(t *testing.T) {
	token := registerAndLogin(t)

	created := httpCreateTask(t, token, "integration task", "test description")
	require.NotEmpty(t, created.ID)
	assert.Equal(t, statusTodo, created.Status)

	listed := httpGetJSON[taskListResponse](t, token, "/tasks?limit=10&offset=0")
	assert.Contains(t, taskIDs(listed.Tasks), created.ID)
	assert.Equal(t, created.ID, httpGetJSON[taskResponse](t, token, "/tasks/"+created.ID).ID)

	updated := httpSend[taskResponse](t, token, http.MethodPut, "/tasks/"+created.ID,
		`{"title":"updated title","description":"updated description"}`, http.StatusOK)
	assert.Equal(t, "updated title", updated.Title)
	assert.Equal(t, "updated title", httpGetJSON[taskResponse](t, token, "/tasks/"+created.ID).Title)

	for _, status := range []string{statusInProgress, "done"} {
		moved := httpSend[taskResponse](t, token, http.MethodPatch, "/tasks/"+created.ID+"/status",
			fmt.Sprintf(`{"status":%q}`, status), http.StatusOK)
		assert.Equal(t, status, moved.Status)
	}

	httpSend[struct{}](t, token, http.MethodDelete, "/tasks/"+created.ID, "", http.StatusNoContent)
	httpSend[struct{}](t, token, http.MethodGet, "/tasks/"+created.ID, "", http.StatusNotFound)
}

// httpSend sends method to /v1 + path with body (none when empty), requires the status want, and
// decodes a 200 answer into T.
func httpSend[T any](t *testing.T, token, method, path, body string, want int) T {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	var reqBody io.Reader = http.NoBody
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	resp, err := doAuthenticatedRequest(ctx, method, basePathV1+path, reqBody, token)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, want, resp.StatusCode, "%s %s", method, path)

	var out T
	if want == http.StatusOK {
		out = parseJSON[T](t, resp)
	}

	return out
}

// HTTP: invalid task status transition is rejected.
func TestHTTPTaskStatusTransitionInvalidV1(t *testing.T) {
	token := registerAndLogin(t)
	task := httpCreateTask(t, token, "transition task", "testing transitions")

	resp, err := httpTransitionTask(t, token, task.ID, "done")
	if err != nil {
		t.Fatalf("Transition to done: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// HTTP: task error cases.
func TestHTTPTaskErrorsV1(t *testing.T) {
	t.Run("no token returns 401", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
		defer cancel()

		body := `{"title":"unauthorized task","description":"should fail"}`

		resp, err := doWebRequestWithTimeout(ctx, http.MethodPost, basePathV1+"/tasks/", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("get non-existent task returns 404", func(t *testing.T) {
		token := registerAndLogin(t)

		ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
		defer cancel()

		resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/tasks/00000000-0000-0000-0000-000000000000", http.NoBody, token)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("create with invalid body returns 422", func(t *testing.T) {
		token := registerAndLogin(t)

		ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
		defer cancel()

		body := `{"description":"missing title"}`

		resp, err := doAuthenticatedRequest(ctx, http.MethodPost, basePathV1+"/tasks/", bytes.NewBufferString(body), token)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("Expected status 422, got %d", resp.StatusCode)
		}
	})
}
