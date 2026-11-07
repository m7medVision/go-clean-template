// Package oidc signs users in through an OpenID Connect provider: the command and queries live in
// command/ and query/; the provider's protocol is internal/repo/sso's, behind repo.SSOProvider.
package oidc

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Deps struct {
	SSO      repo.SSOProvider
	Store    repo.EventStore
	Sessions repo.SessionIssuer
}

func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.FinishOIDCLoginHandler{SSO: d.SSO, Store: d.Store, Sessions: d.Sessions})

	cqrs.HandleQuery(qs, query.BeginOIDCLoginHandler{SSO: d.SSO})
	cqrs.HandleQuery(qs, query.GetOIDCStatusHandler{SSO: d.SSO})
}
