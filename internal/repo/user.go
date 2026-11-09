package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

//go:generate mockgen -source=user.go -destination=repomock/user.go -package=repomock

type (
	// UserReadModel reads the users table, which only the user projector writes. A missing user, or
	// an ID that is not a UUID, is entity.ErrUserNotFound.
	UserReadModel interface {
		ByID(ctx context.Context, id string) (entity.User, error)
		ByEmail(ctx context.Context, email string) (entity.User, error)
	}

	// UserNotifier tells a user about events on their account. The queue-backed implementation
	// (internal/repo/notify) publishes an email for the worker to send.
	UserNotifier interface {
		Welcome(ctx context.Context, user *entity.User) error
	}
)
