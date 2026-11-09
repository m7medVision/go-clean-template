package task

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
)

func NewTaskResponse(task *mongotask.TaskWithAttributes) *v1.TaskResponse {
	return &v1.TaskResponse{
		Attributes:  newAttributes(task.Attributes),
		Id:          task.ID,
		UserId:      task.UserID,
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
		CreatedAt:   response.Time(task.CreatedAt),
		UpdatedAt:   response.Time(task.UpdatedAt),
	}
}

func newAttributes(a mongotask.Attributes) *v1.TaskAttributes {
	out := &v1.TaskAttributes{Estimate: a.Estimate, Labels: a.Labels}
	if a.Priority != nil {
		p := string(*a.Priority)
		out.Priority = &p
	}

	return out
}

func NewListTasksResponse(tasks []mongotask.TaskWithAttributes, total int) *v1.ListTasksResponse {
	pbTasks := make([]*v1.TaskResponse, len(tasks))
	for i := range tasks {
		pbTasks[i] = NewTaskResponse(&tasks[i])
	}

	return &v1.ListTasksResponse{
		Tasks: pbTasks,
		Total: response.Count(total),
	}
}
