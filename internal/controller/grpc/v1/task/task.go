package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateTask -.
func (c *Controller) CreateTask(ctx context.Context, req *v1.CreateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	written, err := cqrs.SendCommand(ctx, c.cmds, command.CreateTask{
		UserID: userID, Title: req.GetTitle(), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "CreateTask")
	}

	resp, err := c.readBack(ctx, userID, written.ID)
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "CreateTask")
	}

	return resp, nil
}

// GetTask -.
func (c *Controller) GetTask(ctx context.Context, req *v1.GetTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetTask")
	}

	return resp, nil
}

// ListTasks -.
func (c *Controller) ListTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	statusFilter, err := listStatus(req)
	if err != nil {
		return nil, err
	}

	return listPage(ctx, c, query.ListTasks{
		UserID: userID, Status: statusFilter, Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListTasks")
}

// ListAllTasks -.
func (c *Controller) ListAllTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	caller, ok := grpcmw.PrincipalFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	statusFilter, err := listStatus(req)
	if err != nil {
		return nil, err
	}

	return listPage(ctx, c, query.ListAllTasks{
		Principal: caller, Status: statusFilter, Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListAllTasks")
}

// listPage sends a list query and answers with its page.
func listPage[Q cqrs.QueryOf[query.TaskPage]](ctx context.Context, c *Controller, q Q, op string) (*v1.ListTasksResponse, error) {
	page, err := cqrs.SendQuery(ctx, c.qs, q)
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, op)
	}

	return NewListTasksResponse(page.Tasks, page.Total), nil
}

// UpdateTask -.
func (c *Controller) UpdateTask(ctx context.Context, req *v1.UpdateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	_, err := cqrs.SendCommand(ctx, c.cmds, command.UpdateTask{
		UserID: userID, TaskID: req.GetId(), Title: req.GetTitle(), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTask")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTask")
	}

	return resp, nil
}

// TransitionTask -.
func (c *Controller) TransitionTask(ctx context.Context, req *v1.TransitionTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	_, err := cqrs.SendCommand(ctx, c.cmds, command.TransitionTask{
		UserID: userID, TaskID: req.GetId(), Status: entity.TaskStatus(req.GetStatus()),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "TransitionTask")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "TransitionTask")
	}

	return resp, nil
}

// DeleteTask -.
func (c *Controller) DeleteTask(ctx context.Context, req *v1.DeleteTaskRequest) (*v1.DeleteTaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	_, err := cqrs.SendCommand(ctx, c.cmds, command.DeleteTask{UserID: userID, TaskID: req.GetId()})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "DeleteTask")
	}

	return &v1.DeleteTaskResponse{}, nil
}
