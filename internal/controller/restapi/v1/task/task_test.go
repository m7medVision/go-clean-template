package task_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTask(t *testing.T) {
	t.Parallel()

	token := resttest.Token(t)

	t.Run("empty title is rejected", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":""}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("invalid json body is rejected", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/tasks", token, "{invalid json}")
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{
			stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
		})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":"my task"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})

	t.Run("trailing slash is stripped", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{
			stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
		})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/tasks/", token, `{"title":"my task"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	token := resttest.Token(t)

	t.Run("not found maps to 404", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{err: entity.ErrTaskNotFound})

		resp := resttest.Do(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("success maps to 204", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeTask{})

		resp := resttest.Do(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}

func TestListAllTasks(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{listed: []entity.Task{
		{ID: "task-1", UserID: "alice", Title: "Alice's", Status: entity.TaskStatusDone},
		{ID: "task-2", UserID: "bob", Title: "Bob's", Status: entity.TaskStatusDone},
	}}
	handler := newRouter(tk)

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/admin/tasks?status=done&limit=5&offset=10", resttest.AdminToken(t), "")

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var body struct {
		Tasks []entity.Task `json:"tasks"`
		Total int           `json:"total"`
	}

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, 2, body.Total)
	assert.Equal(t, []string{"alice", "bob"}, []string{body.Tasks[0].UserID, body.Tasks[1].UserID})

	assert.Equal(t, entity.Principal{UserID: "admin-1", Roles: []entity.Role{entity.RoleAdmin, entity.RoleUser}}, tk.gotCaller)
	require.NotNil(t, tk.gotStatus)
	assert.Equal(t, entity.TaskStatusDone, *tk.gotStatus)
	assert.Equal(t, 5, tk.gotLimit)
	assert.Equal(t, 10, tk.gotOffset)
}

func TestListAllTasksAdminRequiredMapsTo403(t *testing.T) {
	t.Parallel()

	handler := newRouter(&fakeTask{err: entity.ErrAdminRequired})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/admin/tasks", resttest.Token(t), "")

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
}

func TestListAllTasksRequiresAuth(t *testing.T) {
	t.Parallel()

	handler := newRouter(&fakeTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/admin/tasks", "", "")

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTaskForbiddenMapsTo403(t *testing.T) {
	t.Parallel()

	handler := newRouter(&fakeTask{err: entity.ErrTaskForbidden})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", resttest.Token(t), "")
	assert.Equal(t, http.StatusForbidden, resp.Code)
}

func TestInvalidTransitionMapsTo400(t *testing.T) {
	t.Parallel()

	handler := newRouter(&fakeTask{err: entity.ErrInvalidTransition})

	resp := resttest.Do(t.Context(), handler, http.MethodPatch, "/v1/tasks/task-1/status",
		resttest.Token(t), `{"status":"done"}`)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestUsecaseFailureMapsTo500(t *testing.T) {
	t.Parallel()

	handler := newRouter(&fakeTask{err: io.EOF})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks", resttest.Token(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}
