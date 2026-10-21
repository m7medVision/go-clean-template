package v1

import (
	"context"
	"errors"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateTask -.
func (c *TaskController) CreateTask(ctx context.Context, req *v1.CreateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	task, err := c.tk.Create(ctx, userID, req.GetTitle(), req.GetDescription())
	if err != nil {
		c.l.Error(err, "grpc - v1 - CreateTask")

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewTaskResponse(&task), nil
}

// GetTask -.
func (c *TaskController) GetTask(ctx context.Context, req *v1.GetTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	task, err := c.tk.Get(ctx, userID, req.GetId())
	if err != nil {
		c.l.Error(err, "grpc - v1 - GetTask")

		if errors.Is(err, entity.ErrTaskNotFound) {
			return nil, status.Error(codes.NotFound, "task not found")
		}

		if errors.Is(err, entity.ErrTaskForbidden) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewTaskResponse(&task), nil
}

// ListTasks -.
func (c *TaskController) ListTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	statusFilter, err := listStatus(req)
	if err != nil {
		return nil, err
	}

	tasks, total, err := c.tk.List(ctx, userID, statusFilter, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		c.l.Error(err, "grpc - v1 - ListTasks")

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewListTasksResponse(tasks, total), nil
}

// ListAllTasks -.
func (c *TaskController) ListAllTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	caller, ok := grpcmw.PrincipalFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	statusFilter, err := listStatus(req)
	if err != nil {
		return nil, err
	}

	tasks, total, err := c.tk.ListAll(ctx, caller, statusFilter, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		if errors.Is(err, entity.ErrAdminRequired) {
			return nil, status.Error(codes.PermissionDenied, "admin role required")
		}

		c.l.Error(err, "grpc - v1 - ListAllTasks")

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewListTasksResponse(tasks, total), nil
}

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

// UpdateTask -.
func (c *TaskController) UpdateTask(ctx context.Context, req *v1.UpdateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	task, err := c.tk.Update(ctx, userID, req.GetId(), req.GetTitle(), req.GetDescription())
	if err != nil {
		c.l.Error(err, "grpc - v1 - UpdateTask")

		if errors.Is(err, entity.ErrTaskNotFound) {
			return nil, status.Error(codes.NotFound, "task not found")
		}

		if errors.Is(err, entity.ErrTaskForbidden) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewTaskResponse(&task), nil
}

// TransitionTask -.
func (c *TaskController) TransitionTask(ctx context.Context, req *v1.TransitionTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	task, err := c.tk.Transition(ctx, userID, req.GetId(), entity.TaskStatus(req.GetStatus()))
	if err != nil {
		c.l.Error(err, "grpc - v1 - TransitionTask")

		if errors.Is(err, entity.ErrTaskNotFound) {
			return nil, status.Error(codes.NotFound, "task not found")
		}

		if errors.Is(err, entity.ErrTaskForbidden) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		if errors.Is(err, entity.ErrInvalidTransition) {
			return nil, status.Error(codes.InvalidArgument, "invalid status transition")
		}

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewTaskResponse(&task), nil
}

// DeleteTask -.
func (c *TaskController) DeleteTask(ctx context.Context, req *v1.DeleteTaskRequest) (*v1.DeleteTaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	err := c.tk.Delete(ctx, userID, req.GetId())
	if err != nil {
		c.l.Error(err, "grpc - v1 - DeleteTask")

		if errors.Is(err, entity.ErrTaskNotFound) {
			return nil, status.Error(codes.NotFound, "task not found")
		}

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &v1.DeleteTaskResponse{}, nil
}
