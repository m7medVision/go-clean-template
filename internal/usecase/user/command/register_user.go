// Package command holds the user domain's commands, one file each, with their handlers.
package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// RegisterUser registers a user with a password. An email or username already taken is
// entity.ErrUserAlreadyExists. The welcome email follows from the UserRegistered event.
type RegisterUser struct {
	cqrs.Command[entity.Written]

	Username string
	Email    string
	Password string
}

func (c RegisterUser) Validate() error { return entity.ValidateSignUp(c.Username, c.Email, c.Password) }

// RegisterUserHandler's Now is the clock (time.Now when nil).
type RegisterUserHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

func (h RegisterUserHandler) Handle(ctx context.Context, cmd RegisterUser) (entity.Written, error) {
	hash, err := entity.HashPassword(cmd.Password)
	if err != nil {
		return entity.Written{}, fmt.Errorf("RegisterUser: %w", err)
	}

	id := uuid.NewString()
	user, claims := entity.NewUserAggregate(id, cmd.Username, cmd.Email, hash, aggregate.Now(h.Now))

	written, err := aggregate.Save(ctx, h.Store, user, claims...)
	if err != nil {
		return entity.Written{}, fmt.Errorf("RegisterUser - aggregate.Save: %w", err)
	}

	return written, nil
}
