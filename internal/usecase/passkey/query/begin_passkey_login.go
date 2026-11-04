package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
)

// BeginPasskeyLogin returns the request options for a discoverable navigator.credentials.get and the
// token FinishPasskeyLogin needs.
type BeginPasskeyLogin struct{}

// QueryName -.
func (BeginPasskeyLogin) QueryName() string { return "BeginPasskeyLogin" }

// BeginPasskeyLoginHandler handles BeginPasskeyLogin.
type BeginPasskeyLoginHandler struct {
	Ceremony *ceremony.Ceremony
}

// Handle -.
func (h BeginPasskeyLoginHandler) Handle(context.Context, BeginPasskeyLogin) (ceremony.Started, error) {
	started, err := h.Ceremony.BeginLogin()
	if err != nil {
		return ceremony.Started{}, fmt.Errorf("BeginPasskeyLogin: %w", err)
	}

	return started, nil
}
