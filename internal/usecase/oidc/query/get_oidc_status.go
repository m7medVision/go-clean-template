package query

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
)

// GetOIDCStatus reads whether SSO is configured, for the sign-in page to offer it.
type GetOIDCStatus struct{}

// QueryName -.
func (GetOIDCStatus) QueryName() string { return "GetOIDCStatus" }

// GetOIDCStatusHandler handles GetOIDCStatus.
type GetOIDCStatusHandler struct {
	SSO *sso.SSO
}

// Handle -.
func (h GetOIDCStatusHandler) Handle(context.Context, GetOIDCStatus) (bool, error) {
	return h.SSO.Enabled(), nil
}
