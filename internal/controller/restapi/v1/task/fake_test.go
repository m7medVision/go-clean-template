package task_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/task"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeTask answers every task command and query: commands write stored's ID, queries read stored,
// and every one of them fails with err when it is set. It keeps what ListAllTasks was asked.
type fakeTask struct {
	stored mongotask.TaskWithAttributes
	err    error

	gotPatch  mongotask.AttributesPatch
	listed    []mongotask.TaskWithAttributes
	gotCaller entity.Principal
	gotStatus *entity.TaskStatus
	gotLimit  int
	gotOffset int
}

// buses registers the fake's handlers on real buses with no pipeline steps.
func (f *fakeTask) buses() cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()
	write := func(context.Context) (entity.Written, error) { return entity.Written{ID: f.stored.ID}, f.err }

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.CreateTask, entity.Written](
		func(ctx context.Context, _ command.CreateTask) (entity.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.UpdateTask, entity.Written](
		func(ctx context.Context, _ command.UpdateTask) (entity.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.TransitionTask, entity.Written](
		func(ctx context.Context, _ command.TransitionTask) (entity.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.DeleteTask, entity.Written](
		func(ctx context.Context, _ command.DeleteTask) (entity.Written, error) { return write(ctx) }))

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.SetTaskAttributes, entity.Written](
		func(ctx context.Context, c command.SetTaskAttributes) (entity.Written, error) {
			f.gotPatch = c.Patch

			return write(ctx)
		}))

	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, mongotask.TaskWithAttributes](
		func(context.Context, query.GetTask) (mongotask.TaskWithAttributes, error) { return f.stored, f.err }))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListTasks, entity.List[mongotask.TaskWithAttributes]](
		func(context.Context, query.ListTasks) (entity.List[mongotask.TaskWithAttributes], error) {
			return entity.List[mongotask.TaskWithAttributes]{Items: []mongotask.TaskWithAttributes{f.stored}, Total: 1}, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListAllTasks, entity.List[mongotask.TaskWithAttributes]](
		func(_ context.Context, q query.ListAllTasks) (entity.List[mongotask.TaskWithAttributes], error) {
			f.gotCaller, f.gotStatus, f.gotLimit, f.gotOffset = q.Principal, q.Status, q.Limit, q.Offset

			return entity.List[mongotask.TaskWithAttributes]{Items: f.listed, Total: len(f.listed)}, f.err
		}))

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

// newRouter serves the REST router over tk's buses; the use cases a task test never reaches are nil.
func newRouter(tk *fakeTask) http.Handler {
	return resttest.NewRouter(tk.buses(), task.NewRoutes)
}
