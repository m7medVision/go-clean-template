// Package oidc serves the v1 SSO status and sign-in operations over REST.
package oidc

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

type controller struct {
	oc usecase.OIDC
	l  logger.Interface
}

// NewRoutes registers the SSO status and sign-in operations.
func NewRoutes(api huma.API, oc usecase.OIDC, l logger.Interface) {
	r := &controller{oc: oc, l: l}

	r.oidcStatus(api)
	r.beginOIDC(api)
	r.finishOIDC(api)
}
