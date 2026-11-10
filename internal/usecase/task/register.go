package task

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/flags"
)

type Deps struct {
	Store      repo.EventStore
	Tasks      repo.TaskReadModel
	Attributes repo.TaskAttributesStore
	Logger     *slog.Logger
	Flags      *flags.Client
}

func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.CreateTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.UpdateTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.TransitionTaskHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.DeleteTaskHandler{Store: d.Store, Attributes: d.Attributes, Logger: d.Logger})
	cqrs.HandleCommand(cmds, command.SetTaskAttributesHandler{Tasks: d.Tasks, Attributes: d.Attributes, Flags: d.Flags})

	cqrs.HandleQuery(qs, query.GetTaskHandler{Tasks: d.Tasks, Attributes: d.Attributes, Flags: d.Flags})
	cqrs.HandleQuery(qs, query.ListTasksHandler{Tasks: d.Tasks, Attributes: d.Attributes, Flags: d.Flags})
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{Tasks: d.Tasks, Attributes: d.Attributes, Flags: d.Flags})
}
