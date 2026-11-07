package query

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetOIDCStatus reads whether SSO is configured, for the sign-in page to offer it.
type GetOIDCStatus struct {
	cqrs.Query[bool]
}

// GetOIDCStatusHandler handles GetOIDCStatus.
type GetOIDCStatusHandler struct {
	SSO repo.SSOProvider
}

// Handle -.
func (h GetOIDCStatusHandler) Handle(context.Context, GetOIDCStatus) (bool, error) {
	return h.SSO.Enabled(), nil
}
