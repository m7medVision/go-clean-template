package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
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
	high := mongotask.PriorityHigh
	attrs := mongotask.Attributes{Priority: &high}

	ctrl := gomock.NewController(t)
	tasks := repomock.NewMockTaskReadModel(ctrl)
	tasks.EXPECT().Get(gomock.Any(), "t1").Return(want, nil).Times(2)

	store := repomock.NewMockTaskAttributesStore(ctrl)
	store.EXPECT().Get(gomock.Any(), "t1").Return(attrs, nil)

	h := query.GetTaskHandler{Tasks: tasks, Attributes: store}

	got, err := h.Handle(context.Background(), query.GetTask{UserID: "u1", TaskID: "t1"})
	require.NoError(t, err)
	assert.Equal(t, mongotask.TaskWithAttributes{Task: want, Attributes: attrs}, got)

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

func TestListTasksMergesAttributesWithOneCallPerPage(t *testing.T) {
	t.Parallel()

	page := entity.List[entity.Task]{Items: []entity.Task{{ID: "t1"}, {ID: "t2"}}, Total: 7}
	low := mongotask.PriorityLow

	ctrl := gomock.NewController(t)
	tasks := repomock.NewMockTaskReadModel(ctrl)
	tasks.EXPECT().List(gomock.Any(), "u1", nil, entity.Page{Limit: 2}).Return(page, nil)

	store := repomock.NewMockTaskAttributesStore(ctrl)
	store.EXPECT().GetMany(gomock.Any(), []string{"t1", "t2"}).
		Return(map[string]mongotask.Attributes{"t2": {Priority: &low}}, nil).Times(1)

	got, err := query.ListTasksHandler{Tasks: tasks, Attributes: store}.
		Handle(context.Background(), query.ListTasks{UserID: "u1", Limit: 2})

	require.NoError(t, err)
	assert.Equal(t, 7, got.Total)
	assert.Equal(t, []mongotask.TaskWithAttributes{
		{ID: "t1"},
		{ID: "t2", Attributes: mongotask.Attributes{Priority: &low}},
	}, got.Items)
}
