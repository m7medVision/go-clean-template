package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/goccy/go-json"
)

const graphqlURL = httpURL + "/graphql"

type graphqlTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type graphqlResponse struct {
	Data struct {
		CreateTask     graphqlTask `json:"createTask"`
		Task           graphqlTask `json:"task"`
		UpdateTask     graphqlTask `json:"updateTask"`
		TransitionTask graphqlTask `json:"transitionTask"`
		DeleteTask     bool        `json:"deleteTask"`
		Tasks          struct {
			Total int `json:"total"`
		} `json:"tasks"`
	} `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func graphqlDo(t *testing.T, token, query string, vars map[string]any) graphqlResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		t.Fatalf("graphql: marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body), token)
	if err != nil {
		t.Fatalf("graphql: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("graphql: expected 200, got %d", resp.StatusCode)
	}

	return parseJSON[graphqlResponse](t, resp)
}

func TestGraphQLTaskCRUD(t *testing.T) {
	token := registerAndLogin(t)

	created := graphqlDo(t, token,
		`mutation($input: CreateTaskInput!) { createTask(input: $input) { id title status } }`,
		map[string]any{"input": map[string]any{"title": "graphql task", "description": "via graphql"}})
	if len(created.Errors) > 0 {
		t.Fatalf("createTask: %v", created.Errors)
	}

	id := created.Data.CreateTask.ID
	if id == "" || created.Data.CreateTask.Status != "TODO" {
		t.Fatalf("createTask: unexpected task %+v", created.Data.CreateTask)
	}

	got := graphqlDo(t, token, `query($id: ID!) { task(id: $id) { id title } }`, map[string]any{"id": id})
	if got.Data.Task.Title != "graphql task" {
		t.Errorf("task: expected title %q, got %+v (errors %v)", "graphql task", got.Data.Task, got.Errors)
	}

	list := graphqlDo(t, token, `query { tasks(status: TODO) { total } }`, nil)
	if list.Data.Tasks.Total < 1 {
		t.Errorf("tasks: expected total >= 1, got %d (errors %v)", list.Data.Tasks.Total, list.Errors)
	}

	updated := graphqlDo(t, token,
		`mutation($id: ID!) { updateTask(id: $id, input: {title: "renamed"}) { title description } }`,
		map[string]any{"id": id})
	if updated.Data.UpdateTask.Title != "renamed" {
		t.Errorf("updateTask: unexpected task %+v (errors %v)", updated.Data.UpdateTask, updated.Errors)
	}

	deleted := graphqlDo(t, token, `mutation($id: ID!) { deleteTask(id: $id) }`, map[string]any{"id": id})
	if !deleted.Data.DeleteTask {
		t.Errorf("deleteTask: expected true (errors %v)", deleted.Errors)
	}

	missing := graphqlDo(t, token, `query($id: ID!) { task(id: $id) { id } }`, map[string]any{"id": id})
	if len(missing.Errors) == 0 || missing.Errors[0].Extensions["code"] != "NOT_FOUND" {
		t.Errorf("task after delete: expected NOT_FOUND, got %v", missing.Errors)
	}
}

func TestGraphQLTaskTransition(t *testing.T) {
	token := registerAndLogin(t)

	created := graphqlDo(t, token,
		`mutation { createTask(input: {title: "graphql transition"}) { id } }`, nil)

	id := created.Data.CreateTask.ID
	if id == "" {
		t.Fatalf("createTask: %v", created.Errors)
	}

	const transition = `mutation($id: ID!) { transitionTask(id: $id, status: IN_PROGRESS) { status } }`

	moved := graphqlDo(t, token, transition, map[string]any{"id": id})
	if moved.Data.TransitionTask.Status != "IN_PROGRESS" {
		t.Errorf("transitionTask: unexpected task %+v (errors %v)", moved.Data.TransitionTask, moved.Errors)
	}

	repeated := graphqlDo(t, token, transition, map[string]any{"id": id})
	if len(repeated.Errors) == 0 || repeated.Errors[0].Extensions["code"] != "BAD_USER_INPUT" {
		t.Errorf("repeated transitionTask: expected BAD_USER_INPUT, got %v", repeated.Errors)
	}
}
