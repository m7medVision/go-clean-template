package task_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateTaskAttributesPassesEachFieldsStateThrough(t *testing.T) {
	t.Parallel()

	high := mongotask.PriorityHigh
	tk := &fakeTask{stored: mongotask.TaskWithAttributes{
		Task:       entity.Task{ID: "task-1"},
		Attributes: mongotask.Attributes{Priority: &high},
	}}

	resp := resttest.Do(t.Context(), newRouter(tk), http.MethodPatch, "/v1/tasks/task-1/attributes", resttest.Token(t),
		`{"priority":"high","estimate":null}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Equal(t, mongotask.AttributesPatch{
		Priority: mongotask.SetTo(mongotask.PriorityHigh),
		Estimate: mongotask.Cleared[float64](),
	}, tk.gotPatch)

	var body struct {
		ID         string `json:"id"`
		Attributes struct {
			Priority string `json:"priority"`
		} `json:"attributes"`
	}

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "task-1", body.ID)
	assert.Equal(t, "high", body.Attributes.Priority)
}

func TestUpdateTaskAttributesRejects(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"an unknown priority":           `{"priority":"urgent"}`,
		"an estimate over the max":      `{"estimate":500}`,
		"an estimate of the wrong type": `{"estimate":"soon"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp := resttest.Do(t.Context(), newRouter(&fakeTask{}), http.MethodPatch, "/v1/tasks/task-1/attributes",
				resttest.Token(t), body)
			assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
		})
	}
}

func TestUpdateTaskAttributesForbiddenMapsTo403(t *testing.T) {
	t.Parallel()

	resp := resttest.Do(t.Context(), newRouter(&fakeTask{err: entity.ErrTaskForbidden}), http.MethodPatch,
		"/v1/tasks/task-1/attributes", resttest.Token(t), `{}`)
	assert.Equal(t, http.StatusForbidden, resp.Code)
}

func TestUpdateTaskAttributesClearsWithNull(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: mongotask.TaskWithAttributes{Task: entity.Task{ID: "task-1"}}}

	resp := resttest.Do(t.Context(), newRouter(tk), http.MethodPatch, "/v1/tasks/task-1/attributes", resttest.Token(t),
		`{"priority":null,"labels":null}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Equal(t, mongotask.AttributesPatch{
		Priority: mongotask.Cleared[mongotask.Priority](),
		Labels:   mongotask.Cleared[[]string](),
	}, tk.gotPatch)
}
