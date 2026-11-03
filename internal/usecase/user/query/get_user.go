package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// GetUser reads a user: entity.ErrUserNotFound when there is none.
type GetUser struct {
	UserID string
}

// QueryName -.
func (GetUser) QueryName() string { return "GetUser" }

// GetUserHandler handles GetUser.
type GetUserHandler struct {
	Users repo.UserReadModel
}

// Handle -.
func (h GetUserHandler) Handle(ctx context.Context, q GetUser) (entity.User, error) {
	user, err := h.Users.ByID(ctx, q.UserID)
	if err != nil {
		return entity.User{}, fmt.Errorf("GetUser - h.Users.ByID: %w", err)
	}

	return user, nil
}
