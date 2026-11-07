package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
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

func TestListTasksPagesTheUsersTasks(t *testing.T) {
	t.Parallel()

	status := entity.TaskStatusDone
	tasks := readModel(t)
	want := []entity.Task{{ID: taskID}}
	tasks.EXPECT().List(gomock.Any(), owner, repo.TaskFilter{Status: &status, Limit: 5, Offset: 10}).Return(want, 11, nil)

	page, err := query.ListTasksHandler{Tasks: tasks}.Handle(context.Background(),
		query.ListTasks{UserID: owner, Status: &status, Limit: 5, Offset: 10})

	require.NoError(t, err)
	assert.Equal(t, query.TaskPage{Tasks: want, Total: 11}, page)
}

func TestListTasksDefaultsThePage(t *testing.T) {
	t.Parallel()

	tasks := readModel(t)
	tasks.EXPECT().List(gomock.Any(), owner, repo.TaskFilter{Limit: 10, Offset: 0}).Return(nil, 0, nil)

	_, err := query.ListTasksHandler{Tasks: tasks}.Handle(context.Background(),
		query.ListTasks{UserID: owner, Limit: 0, Offset: -3})

	require.NoError(t, err)
}

func TestListAllTasksPagesEveryUsersTasks(t *testing.T) {
	t.Parallel()

	tasks := readModel(t)
	want := []entity.Task{{ID: taskID}, {ID: "task-2"}}
	tasks.EXPECT().ListAll(gomock.Any(), repo.TaskFilter{Limit: 10}).Return(want, 2, nil)

	page, err := query.ListAllTasksHandler{Tasks: tasks}.Handle(context.Background(), query.ListAllTasks{})

	require.NoError(t, err)
	assert.Equal(t, query.TaskPage{Tasks: want, Total: 2}, page)
}

func TestListAllTasksIsForAdmins(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{})

	user := entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}

	_, err := cqrs.SendQuery(context.Background(), qs, query.ListAllTasks{Principal: user})
	require.ErrorIs(t, err, entity.ErrAdminRequired)
}
