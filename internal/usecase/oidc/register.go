// Package oidc signs users in through an OpenID Connect provider: the command and queries live in
// command/ and query/, and the sign-in ceremony they share in sso/.
package oidc

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// Deps is what the OIDC handlers are built from.
type Deps struct {
	SSO    *sso.SSO
	Store  repo.EventStore
	Tokens *jwt.Manager
}

// Register registers the OIDC command and query handlers, one line each.
func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.FinishOIDCLoginHandler{SSO: d.SSO, Store: d.Store, Tokens: d.Tokens})

	cqrs.HandleQuery(qs, query.BeginOIDCLoginHandler{SSO: d.SSO})
	cqrs.HandleQuery(qs, query.GetOIDCStatusHandler{SSO: d.SSO})
}
