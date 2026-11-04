package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeTask answers every task command and query on real buses. stored is the task: commands
// change it as the aggregate would and GetTask reads it back. Every message fails with err when it
// is set, and called records that one reached a handler.
type fakeTask struct {
	stored     entity.Task
	err        error
	called     bool
	listStatus *entity.TaskStatus
	listCaller entity.Principal
}

// write runs change on stored, as a command handler would, unless err is set.
func (f *fakeTask) write(change func(t *entity.Task)) (cqrs.Written, error) {
	f.called = true
	if f.err != nil {
		return cqrs.Written{}, f.err
	}

	change(&f.stored)

	return cqrs.Written{ID: f.stored.ID}, nil
}

func (f *fakeTask) buses() cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.CreateTask, cqrs.Written](
		func(_ context.Context, c command.CreateTask) (cqrs.Written, error) {
			return f.write(func(t *entity.Task) {
				*t = entity.Task{
					ID: "task-1", UserID: c.UserID, Title: c.Title, Description: c.Description, Status: entity.TaskStatusTodo,
				}
			})
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.UpdateTask, cqrs.Written](
		func(_ context.Context, c command.UpdateTask) (cqrs.Written, error) {
			return f.write(func(t *entity.Task) { t.Title, t.Description = c.Title, c.Description })
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.TransitionTask, cqrs.Written](
		func(_ context.Context, c command.TransitionTask) (cqrs.Written, error) {
			return f.write(func(t *entity.Task) { t.Status = c.Status })
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.DeleteTask, cqrs.Written](
		func(context.Context, command.DeleteTask) (cqrs.Written, error) {
			return f.write(func(*entity.Task) {})
		}))

	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) {
			f.called = true

			return f.stored, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListTasks, query.TaskPage](
		func(_ context.Context, q query.ListTasks) (query.TaskPage, error) {
			f.called, f.listStatus = true, q.Status

			return query.TaskPage{Tasks: []entity.Task{f.stored}, Total: 1}, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListAllTasks, query.TaskPage](
		func(_ context.Context, q query.ListAllTasks) (query.TaskPage, error) {
			f.called, f.listCaller, f.listStatus = true, q.Principal, q.Status

			return query.TaskPage{Tasks: []entity.Task{f.stored}, Total: 1}, f.err
		}))

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

// do sends gql to the GraphQL handler over tk, signed in as graphqltest.UserID when authenticated.
func do(t *testing.T, tk *fakeTask, authenticated bool, gql string, vars map[string]any) graphqltest.Response {
	t.Helper()

	token := ""
	if authenticated {
		token = graphqltest.Token(t)
	}

	return graphqltest.Post(t, tk.buses(), token, gql, vars)
}

// doAs sends gql signed in as graphqltest.UserID with roles.
func doAs(t *testing.T, tk *fakeTask, roles []string, gql string) graphqltest.Response {
	t.Helper()

	return graphqltest.Post(t, tk.buses(), graphqltest.Token(t, roles...), gql, nil)
}
