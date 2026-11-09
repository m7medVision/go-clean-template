package task

import (
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
)

type TaskAttributes struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	Priority *mongotask.Priority `json:"priority,omitempty" enum:"low,medium,high" example:"high"`
	Estimate *float64            `json:"estimate,omitempty" minimum:"0.5" maximum:"200" example:"3.5"`
	Labels   []string            `json:"labels,omitempty" maxItems:"10" uniqueItems:"true" example:"[\"backend\"]"`
}

type Task struct {
	entity.Task

	Attributes TaskAttributes `json:"attributes"`
}

type TaskList struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	Tasks []Task `json:"tasks"`
	Total int    `json:"total" example:"42"`
}

type taskOutput struct {
	Body Task
}

type listTasksOutput struct {
	Body TaskList
}

func newTask(t *mongotask.TaskWithAttributes) Task {
	return Task{Task: t.Task, Attributes: TaskAttributes(t.Attributes)}
}

func newTaskList(page entity.List[mongotask.TaskWithAttributes]) TaskList {
	tasks := make([]Task, len(page.Items))
	for i := range page.Items {
		tasks[i] = newTask(&page.Items[i])
	}

	return TaskList{Tasks: tasks, Total: page.Total}
}
