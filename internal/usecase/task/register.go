package task

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Deps is what the task handlers are built from.
type Deps struct {
	Store repo.EventStore
	Tasks repo.TaskReadModel
}

// Register registers every task command and query handler, one line each.
func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.CreateTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.UpdateTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.TransitionTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.DeleteTaskHandler{Store: d.Store})

	cqrs.HandleQuery(qs, query.GetTaskHandler{Tasks: d.Tasks})
	cqrs.HandleQuery(qs, query.ListTasksHandler{Tasks: d.Tasks})
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{Tasks: d.Tasks})
}
