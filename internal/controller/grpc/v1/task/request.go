package task

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// listStatus is the status filter req asks for, or nil for every status.
func listStatus(req *v1.ListTasksRequest) (*entity.TaskStatus, error) {
	if req.GetStatus() == "" {
		return nil, nil //nolint:nilnil // no filter is not an error
	}

	s := entity.TaskStatus(req.GetStatus())
	if !s.Valid() {
		return nil, status.Error(codes.InvalidArgument, "invalid task status")
	}

	return &s, nil
}
