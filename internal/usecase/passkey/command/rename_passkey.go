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
	UserID string
	ID     string
	Name   string
}

// CommandName -.
func (RenamePasskey) CommandName() string { return "RenamePasskey" }

// ActorID -.
func (c RenamePasskey) ActorID() string { return c.UserID }

// RenamePasskeyHandler handles RenamePasskey. Now is the clock (time.Now when nil).
type RenamePasskeyHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h RenamePasskeyHandler) Handle(ctx context.Context, cmd RenamePasskey) (cqrs.Written, error) {
	var id entity.PasskeyID
	if err := id.UnmarshalText([]byte(cmd.ID)); err != nil {
		return cqrs.Written{}, fmt.Errorf("RenamePasskey: %w", err)
	}

	written, err := aggregate.Change(ctx, h.Store, id.String(), entity.PasskeyStreamID(id), &entity.PasskeyAggregate{},
		entity.ErrPasskeyNotFound, func(p *entity.PasskeyAggregate) error {
			return p.Rename(cmd.UserID, entity.PasskeyName(cmd.Name), aggregate.Now(h.Now))
		})
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("RenamePasskey: %w", err)
	}

	return written, nil
}
