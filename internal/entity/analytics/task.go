package analytics

import "github.com/m7medvision/go-clean-template/internal/entity"

type TaskCreated struct{}

type TaskStatusChanged struct {
	From entity.TaskStatus `json:"from"`
	To   entity.TaskStatus `json:"to"`
}

func (TaskCreated) Name() string { return "task_created" }

func (TaskStatusChanged) Name() string { return "task_status_changed" }

func taskEvents() []Properties {
	return []Properties{TaskCreated{}, TaskStatusChanged{}}
}
