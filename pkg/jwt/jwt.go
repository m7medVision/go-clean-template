package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	secret   string
	duration time.Duration
}

func New(secret string, duration time.Duration) *Manager {
	return &Manager{
		secret:   secret,
		duration: duration,
	}
}

type tokenClaims struct {
	jwtlib.RegisteredClaims

	Roles []string `json:"roles,omitempty"`
}

// GenerateToken creates a new JWT token for the given user ID, carrying roles in a roles claim.
func (m *Manager) GenerateToken(userID string, roles []string) (string, error) {
	now := time.Now()

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, tokenClaims{
		Subject:   userID,
		IssuedAt:  jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(m.duration)),
		Roles:     roles,
	})

	tokenString, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("jwt - GenerateToken - token.SignedString: %w", err)
	}

	return tokenString, nil
}

// Claims is what a valid auth token says about its caller.
type Claims struct {
	UserID string
	// IssuedAt is when the token was issued: when the caller last proved who they are. It is zero
	// for a token without an iat claim.
	IssuedAt time.Time
	// Roles are the role names the token was issued with, unchecked: callers keep the ones they know.
	Roles []string
}

// Parse validates a JWT token, signed with HS256 as GenerateToken signs it, and returns its claims.
func (m *Manager) Parse(tokenString string) (Claims, error) {
	var parsed tokenClaims

	token, err := jwtlib.ParseWithClaims(tokenString, &parsed, func(*jwtlib.Token) (any, error) {
		return []byte(m.secret), nil
	}, jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}))
	if err != nil {
		return Claims{}, fmt.Errorf("jwt - Parse - jwtlib.Parse: %w", err)
	}

	sub, err := token.Claims.GetSubject()
	if err != nil {
		return Claims{}, fmt.Errorf("jwt - Parse - GetSubject: %w", err)
	}

	claims := Claims{UserID: sub, Roles: parsed.Roles}

	issuedAt, err := token.Claims.GetIssuedAt()
	if err != nil {
		return Claims{}, fmt.Errorf("jwt - Parse - GetIssuedAt: %w", err)
	}

	if issuedAt != nil {
		claims.IssuedAt = issuedAt.Time
	}

	return claims, nil
}
