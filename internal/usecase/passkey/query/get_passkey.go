package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetPasskey reads UserID's passkey ID (base64url): what a transport answers a passkey command with.
// Any other ID, including another user's passkey, is entity.ErrPasskeyNotFound.
type GetPasskey struct {
	cqrs.Query[entity.Passkey]

	UserID string
	ID     string
}

type GetPasskeyHandler struct {
	Passkeys repo.PasskeyReadModel
}

func (h GetPasskeyHandler) Handle(ctx context.Context, q GetPasskey) (entity.Passkey, error) {
	var id entity.PasskeyID
	if err := id.UnmarshalText([]byte(q.ID)); err != nil {
		return entity.Passkey{}, fmt.Errorf("GetPasskey: %w", err)
	}

	passkey, err := h.Passkeys.Get(ctx, id)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("GetPasskey - h.Passkeys.Get: %w", err)
	}

	if err = passkey.ReadableBy(q.UserID); err != nil {
		return entity.Passkey{}, fmt.Errorf("GetPasskey: %w", err)
	}

	return passkey, nil
}
