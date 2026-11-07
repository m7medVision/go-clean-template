package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetUser reads a user: entity.ErrUserNotFound when there is none.
type GetUser struct {
	cqrs.Query[entity.User]

	UserID string
}

type GetUserHandler struct {
	Users repo.UserReadModel
}

func (h GetUserHandler) Handle(ctx context.Context, q GetUser) (entity.User, error) {
	user, err := h.Users.ByID(ctx, q.UserID)
	if err != nil {
		return entity.User{}, fmt.Errorf("GetUser - h.Users.ByID: %w", err)
	}

	return user, nil
}
