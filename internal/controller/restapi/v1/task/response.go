package task

import "github.com/m7medvision/go-clean-template/internal/entity"

// TaskList -.
type TaskList struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	Tasks []entity.Task `json:"tasks"`
	Total int           `json:"total" example:"42"`
}

// taskOutput is the task an operation wrote or read.
type taskOutput struct {
	Body entity.Task
}

type listTasksOutput struct {
	Body TaskList
}
