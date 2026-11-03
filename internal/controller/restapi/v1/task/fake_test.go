package task_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeTask answers every task command and query: commands write stored's ID, queries read stored,
// and every one of them fails with err when it is set. It keeps what ListAllTasks was asked.
type fakeTask struct {
	stored entity.Task
	err    error

	listed    []entity.Task
	gotCaller entity.Principal
	gotStatus *entity.TaskStatus
	gotLimit  int
	gotOffset int
}

// buses registers the fake's handlers on real buses with no pipeline steps.
func (f *fakeTask) buses() cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()
	write := func(context.Context) (cqrs.Written, error) { return cqrs.Written{ID: f.stored.ID}, f.err }

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.CreateTask, cqrs.Written](
		func(ctx context.Context, _ command.CreateTask) (cqrs.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.UpdateTask, cqrs.Written](
		func(ctx context.Context, _ command.UpdateTask) (cqrs.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.TransitionTask, cqrs.Written](
		func(ctx context.Context, _ command.TransitionTask) (cqrs.Written, error) { return write(ctx) }))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.DeleteTask, cqrs.Written](
		func(ctx context.Context, _ command.DeleteTask) (cqrs.Written, error) { return write(ctx) }))

	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) { return f.stored, f.err }))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListTasks, query.TaskPage](
		func(context.Context, query.ListTasks) (query.TaskPage, error) {
			return query.TaskPage{Tasks: []entity.Task{f.stored}, Total: 1}, f.err
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.ListAllTasks, query.TaskPage](
		func(_ context.Context, q query.ListAllTasks) (query.TaskPage, error) {
			f.gotCaller, f.gotStatus, f.gotLimit, f.gotOffset = q.Principal, q.Status, q.Limit, q.Offset

			return query.TaskPage{Tasks: f.listed, Total: len(f.listed)}, f.err
		}))

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

// newRouter serves the REST router over tk's buses; the use cases a task test never reaches are nil.
func newRouter(cfg *config.Config, tk *fakeTask) http.Handler {
	return resttest.NewRouter(cfg, tk.buses(), nil)
}
