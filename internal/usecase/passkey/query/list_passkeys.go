package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// ListPasskeys reads UserID's passkeys, oldest first.
type ListPasskeys struct {
	cqrs.Query[[]entity.Passkey]

	UserID string
}

// ListPasskeysHandler handles ListPasskeys.
type ListPasskeysHandler struct {
	Passkeys repo.PasskeyReadModel
}

// Handle -.
func (h ListPasskeysHandler) Handle(ctx context.Context, q ListPasskeys) ([]entity.Passkey, error) {
	passkeys, err := h.Passkeys.ListByUser(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("ListPasskeys - h.Passkeys.ListByUser: %w", err)
	}

	return passkeys, nil
}
