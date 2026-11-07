package task

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// listStatus is the status filter req asks for, or nil for every status.
func listStatus(req *v1.ListTasksRequest) *entity.TaskStatus {
	if req.GetStatus() == "" {
		return nil
	}

	s := entity.TaskStatus(req.GetStatus())

	return &s
}
