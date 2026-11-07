package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	owner  = "user-1"
	taskID = "task-1"
)

func readModel(t *testing.T) *repomock.MockTaskReadModel {
	t.Helper()

	return repomock.NewMockTaskReadModel(gomock.NewController(t))
}

func TestGetTaskReadsTheReadModel(t *testing.T) {
	t.Parallel()

	tasks := readModel(t)
	want := entity.Task{ID: taskID, UserID: owner, Title: "Write"}
	tasks.EXPECT().Get(gomock.Any(), owner, taskID).Return(want, nil)

	got, err := query.GetTaskHandler{Tasks: tasks}.Handle(context.Background(), query.GetTask{UserID: owner, TaskID: taskID})

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetTaskKeepsTheReadModelsError(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{entity.ErrTaskNotFound, entity.ErrTaskForbidden} {
		tasks := readModel(t)
		tasks.EXPECT().Get(gomock.Any(), owner, taskID).Return(entity.Task{}, sentinel)

		_, err := query.GetTaskHandler{Tasks: tasks}.Handle(context.Background(), query.GetTask{UserID: owner, TaskID: taskID})

		require.ErrorIs(t, err, sentinel)
	}
}

func TestListAllTasksIsForAdmins(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{})

	user := entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}

	_, err := cqrs.SendQuery(context.Background(), qs, query.ListAllTasks{Principal: user})
	require.ErrorIs(t, err, entity.ErrAdminRequired)
}
