package command

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// RenamePasskey names UserID's passkey ID (base64url): trimmed, and a blank name the default. Any
// other ID, including another user's passkey, is entity.ErrPasskeyNotFound.
type RenamePasskey struct {
	cqrs.Command[entity.Written]

	UserID string
	ID     string
	Name   string
}

func (c RenamePasskey) ActorID() string { return c.UserID }

// RenamePasskeyHandler's Now is the clock (time.Now when nil).
type RenamePasskeyHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

func (h RenamePasskeyHandler) Handle(ctx context.Context, cmd RenamePasskey) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.ID, func(p *entity.PasskeyAggregate) error {
		return p.Rename(cmd.UserID, cmd.Name, aggregate.Now(h.Now))
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("RenamePasskey: %w", err)
	}

	return written, nil
}
