package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// BeginPasskeyLogin returns the request options for a discoverable navigator.credentials.get and the
// token FinishPasskeyLogin needs.
type BeginPasskeyLogin struct {
	cqrs.Query[entity.CeremonyStarted]
}

type BeginPasskeyLoginHandler struct {
	Ceremony repo.PasskeyCeremony
}

func (h BeginPasskeyLoginHandler) Handle(context.Context, BeginPasskeyLogin) (entity.CeremonyStarted, error) {
	started, err := h.Ceremony.BeginLogin()
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("BeginPasskeyLogin: %w", err)
	}

	return started, nil
}
