package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/require"
)

func TestListAllTasksIsForAdmins(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, query.ListAllTasksHandler{})

	user := entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}

	_, err := cqrs.SendQuery(context.Background(), qs, query.ListAllTasks{Principal: user})
	require.ErrorIs(t, err, entity.ErrAdminRequired)
}
