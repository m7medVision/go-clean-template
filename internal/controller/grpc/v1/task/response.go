package task

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// NewTaskResponse -.
func NewTaskResponse(task *entity.Task) *v1.TaskResponse {
	return &v1.TaskResponse{
		Id:          task.ID,
		UserId:      task.UserID,
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
		CreatedAt:   response.Time(task.CreatedAt),
		UpdatedAt:   response.Time(task.UpdatedAt),
	}
}

// NewListTasksResponse -.
func NewListTasksResponse(tasks []entity.Task, total int) *v1.ListTasksResponse {
	pbTasks := make([]*v1.TaskResponse, len(tasks))
	for i := range tasks {
		pbTasks[i] = NewTaskResponse(&tasks[i])
	}

	return &v1.ListTasksResponse{
		Tasks: pbTasks,
		Total: response.Count(total),
	}
}
