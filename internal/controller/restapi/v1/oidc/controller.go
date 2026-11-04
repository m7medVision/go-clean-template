// Package oidc serves the v1 SSO status and sign-in operations over REST.
package oidc

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    logger.Interface
}

// NewRoutes registers the SSO status and sign-in operations.
func NewRoutes(api huma.API, b cqrs.Buses, l logger.Interface) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.oidcStatus(api)
	r.beginOIDC(api)
	r.finishOIDC(api)
}
