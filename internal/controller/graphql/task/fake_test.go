package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeTask answers every task command and query on real buses. stored is the task: commands
// change it as the aggregate would and GetTask reads it back. Every message fails with err when it
// is set, and called records that one reached a handler.
type fakeTask struct {
	stored     entity.Task
	attrs      mongotask.Attributes
	gotPatch   mongotask.AttributesPatch
	err        error
	called     bool
	listStatus *entity.TaskStatus
	listCaller entity.Principal
}

// write runs change on stored, as a command handler would, unless err is set.
func (f *fakeTask) write(change func(t *entity.Task)) (entity.Written, error) {
	f.called = true
	if f.err != nil {
		return entity.Written{}, f.err
	}

	change(&f.stored)

	return entity.Written{ID: f.stored.ID}, nil
}

func (f *fakeTask) read() mongotask.TaskWithAttributes {
	return mongotask.TaskWithAttributes{Task: f.stored, Attributes: f.attrs}
}

func (f *fakeTask) buses() cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.CreateTask, entity.Written](
		func(_ context.Context, c command.CreateTask) (entity.Written, error) {
			return f.write(func(t *entity.Task) {
				*t = entity.Task{
					ID: "task-1", UserID: c.UserID, Title: c.Title, Description: c.Description, Status: entity.TaskStatusTodo,
				}
			})
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.UpdateTask, entity.Written](
		func(_ context.Context, c command.UpdateTask) (entity.Written, error) {
			return f.write(func(t *entity.Task) { t.Title, t.Description = c.Title, c.Description })
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.TransitionTask, entity.Written](
		func(_ context.Context, c command.TransitionTask) (entity.Written, error) {
			return f.write(func(t *entity.Task) { t.Status = c.Status })
		}))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.DeleteTask, entity.Written](
		func(context.Context, command.DeleteTask) (entity.Written, error) {
			return f.write(func(*entity.Task) {})
		}))

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.SetTaskAttributes, entity.Written](
		func(_ context.Context, c command.SetTaskAttributes) (entity.Written, error) {
			f.gotPatch = c.Patch

			return f.write(func(*entity.Task) {})
		}))

	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, mongotask.TaskWithAttributes](
		func(context.Context, query.GetTask) (mongotask.TaskWithAttributes, error) {
			f.called = true

			return f.read(), f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListTasks, entity.List[mongotask.TaskWithAttributes]](
		func(_ context.Context, q query.ListTasks) (entity.List[mongotask.TaskWithAttributes], error) {
			f.called, f.listStatus = true, q.Status

			return entity.List[mongotask.TaskWithAttributes]{Items: []mongotask.TaskWithAttributes{f.read()}, Total: 1}, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListAllTasks, entity.List[mongotask.TaskWithAttributes]](
		func(_ context.Context, q query.ListAllTasks) (entity.List[mongotask.TaskWithAttributes], error) {
			f.called, f.listCaller, f.listStatus = true, q.Principal, q.Status

			return entity.List[mongotask.TaskWithAttributes]{Items: []mongotask.TaskWithAttributes{f.read()}, Total: 1}, f.err
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
