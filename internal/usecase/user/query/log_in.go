// Package query holds the user domain's queries, one file each, with their handlers. They read
// only the user read model.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// LogIn checks a password and returns an auth token with the local roles. Every wrong credential
// is entity.ErrInvalidCredentials alike, so a caller cannot tell which emails exist. It changes
// nothing, so it is a query.
type LogIn struct {
	cqrs.Query[entity.SignedIn]

	Email    string
	Password string
}

type LogInHandler struct {
	Users    repo.UserReadModel
	Sessions repo.SessionIssuer
}

func (h LogInHandler) Handle(ctx context.Context, q LogIn) (entity.SignedIn, error) {
	user, err := h.Users.ByEmail(ctx, q.Email)
	if err != nil || !user.PasswordMatches(q.Password) {
		return entity.SignedIn{}, entity.ErrInvalidCredentials
	}

	token, err := h.Sessions.Issue(user.ID, entity.LocalRoles())
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("LogIn - h.Sessions.Issue: %w", err)
	}

	return entity.SignedIn{ID: user.ID, Token: token}, nil
}
