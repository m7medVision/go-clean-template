// Package query holds the user domain's queries, one file each, with their handlers. They read
// only the user read model.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

// LogIn checks a password and returns an auth token with the local roles. Every wrong credential
// is entity.ErrInvalidCredentials alike, so a caller cannot tell which emails exist. It changes
// nothing, so it is a query; the token is its only result.
type LogIn struct {
	Email    string
	Password string
}

// QueryName -.
func (LogIn) QueryName() string { return "LogIn" }

// LogInHandler handles LogIn.
type LogInHandler struct {
	Users  repo.UserReadModel
	Tokens *jwt.Manager
}

// Handle -.
func (h LogInHandler) Handle(ctx context.Context, q LogIn) (string, error) {
	user, err := h.Users.ByEmail(ctx, q.Email)
	// A user stored without a password hash has no password to sign in with.
	if err != nil || user.PasswordHash == "" {
		return "", entity.ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(q.Password)); err != nil {
		return "", entity.ErrInvalidCredentials
	}

	token, err := h.Tokens.GenerateToken(user.ID, entity.RoleNames(entity.LocalRoles()))
	if err != nil {
		return "", fmt.Errorf("LogIn - h.Tokens.GenerateToken: %w", err)
	}

	return token, nil
}
