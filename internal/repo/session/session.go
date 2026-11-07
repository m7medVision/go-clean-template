// Package session issues the auth token a sign-in answers with, as a JWT signed by pkg/jwt.
package session

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// Issuer signs auth tokens with the app's JWT key.
type Issuer struct {
	tokens *jwt.Manager
}

var _ repo.SessionIssuer = Issuer{}

// New returns the issuer of tokens signed by tokens.
func New(tokens *jwt.Manager) Issuer {
	return Issuer{tokens: tokens}
}

// Issue -.
func (i Issuer) Issue(userID string, roles []entity.Role) (string, error) {
	token, err := i.tokens.GenerateToken(userID, entity.RoleNames(roles))
	if err != nil {
		return "", fmt.Errorf("session - Issue: %w", err)
	}

	return token, nil
}
