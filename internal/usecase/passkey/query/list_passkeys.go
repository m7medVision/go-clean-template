package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// ListPasskeys reads UserID's passkeys, oldest first.
type ListPasskeys struct {
	UserID string
}

// QueryName -.
func (ListPasskeys) QueryName() string { return "ListPasskeys" }

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
