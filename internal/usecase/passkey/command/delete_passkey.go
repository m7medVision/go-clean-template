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

// DeletePasskey deletes UserID's passkey ID (base64url): it can no longer sign in. Deleting the last
// passkey is allowed, since every account still has its password. Any other ID, including another
// user's passkey, is entity.ErrPasskeyNotFound.
type DeletePasskey struct {
	cqrs.Command[entity.Written]

	UserID string
	ID     string
}

func (c DeletePasskey) ActorID() string { return c.UserID }

// DeletePasskeyHandler's Now is the clock (time.Now when nil).
type DeletePasskeyHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

func (h DeletePasskeyHandler) Handle(ctx context.Context, cmd DeletePasskey) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.ID, func(p *entity.PasskeyAggregate) error {
		return p.Delete(cmd.UserID, aggregate.Now(h.Now))
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("DeletePasskey: %w", err)
	}

	return written, nil
}
