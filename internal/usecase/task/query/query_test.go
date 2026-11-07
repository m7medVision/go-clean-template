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

func TestGetTaskReadsTheCallersTaskOnly(t *testing.T) {
	t.Parallel()

	want := entity.Task{ID: "t1", UserID: "u1", Title: "Write"}
	tasks := repomock.NewMockTaskReadModel(gomock.NewController(t))
	tasks.EXPECT().Get(gomock.Any(), "t1").Return(want, nil).Times(2)
	h := query.GetTaskHandler{Tasks: tasks}

	got, err := h.Handle(context.Background(), query.GetTask{UserID: "u1", TaskID: "t1"})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = h.Handle(context.Background(), query.GetTask{UserID: "u2", TaskID: "t1"})
	require.ErrorIs(t, err, entity.ErrTaskForbidden)
}

func TestListAllTasksIsForAdmins(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{})

	user := entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}

	_, err := cqrs.SendQuery(context.Background(), qs, query.ListAllTasks{Principal: user})
	require.ErrorIs(t, err, entity.ErrAdminRequired)
}
