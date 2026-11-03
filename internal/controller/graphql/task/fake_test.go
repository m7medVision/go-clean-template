package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
)

type fakeTask struct {
	usecase.Task

	stored     entity.Task
	err        error
	called     bool
	listStatus *entity.TaskStatus
	listCaller entity.Principal
}

func (f *fakeTask) Create(_ context.Context, userID, title, description string) (entity.Task, error) {
	f.called = true

	return entity.Task{
		ID: "task-1", UserID: userID, Title: title, Description: description, Status: entity.TaskStatusTodo,
	}, f.err
}

func (f *fakeTask) Get(context.Context, string, string) (entity.Task, error) {
	f.called = true

	return f.stored, f.err
}

func (f *fakeTask) List(_ context.Context, _ string, status *entity.TaskStatus, _, _ int) ([]entity.Task, int, error) {
	f.called = true
	f.listStatus = status

	return []entity.Task{f.stored}, 1, f.err
}

func (f *fakeTask) ListAll(
	_ context.Context, caller entity.Principal, status *entity.TaskStatus, _, _ int,
) ([]entity.Task, int, error) {
	f.called = true
	f.listCaller, f.listStatus = caller, status

	return []entity.Task{f.stored}, 1, f.err
}

func (f *fakeTask) Update(_ context.Context, _, _, title, description string) (entity.Task, error) {
	f.called = true

	updated := f.stored
	updated.Title = title
	updated.Description = description

	return updated, f.err
}

func (f *fakeTask) Transition(_ context.Context, _, _ string, status entity.TaskStatus) (entity.Task, error) {
	f.called = true

	moved := f.stored
	moved.Status = status

	return moved, f.err
}

func (f *fakeTask) Delete(context.Context, string, string) error {
	f.called = true

	return f.err
}

// do sends query to the GraphQL handler over tk, signed in as graphqltest.UserID when authenticated.
func do(t *testing.T, tk *fakeTask, authenticated bool, query string, vars map[string]any) graphqltest.Response {
	t.Helper()

	token := ""
	if authenticated {
		token = graphqltest.Token(t)
	}

	return graphqltest.Post(t, tk, nil, token, query, vars)
}

// doAs sends query signed in as graphqltest.UserID with roles.
func doAs(t *testing.T, tk *fakeTask, roles []string, query string) graphqltest.Response {
	t.Helper()

	return graphqltest.Post(t, tk, nil, graphqltest.Token(t, roles...), query, nil)
}
