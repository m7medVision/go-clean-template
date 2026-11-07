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
	"golang.org/x/crypto/bcrypt"
)

// RegisterUser registers a user with a password. An email or username already taken is
// entity.ErrUserAlreadyExists. The welcome email follows from the UserRegistered event.
type RegisterUser struct {
	cqrs.Command[cqrs.Written]

	Username string
	Email    string
	Password string
}

// RegisterUserHandler handles RegisterUser. Now is the clock (time.Now when nil).
type RegisterUserHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h RegisterUserHandler) Handle(ctx context.Context, cmd RegisterUser) (cqrs.Written, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("RegisterUser - bcrypt.GenerateFromPassword: %w", err)
	}

	id := uuid.NewString()
	user, claims := entity.NewUserAggregate(id, cmd.Username, cmd.Email, string(hash), aggregate.Now(h.Now))

	written, err := aggregate.Create(ctx, h.Store, id, user, claims...)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("RegisterUser - aggregate.Create: %w", err)
	}

	return written, nil
}
