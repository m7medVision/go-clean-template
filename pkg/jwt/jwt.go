package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// ErrUnexpectedSigningMethod is returned when the JWT signing method is not expected.
var ErrUnexpectedSigningMethod = errors.New("unexpected signing method")

// Manager handles JWT token generation and parsing.
type Manager struct {
	secret   string
	duration time.Duration
}

// New -.
func New(secret string, duration time.Duration) *Manager {
	return &Manager{
		secret:   secret,
		duration: duration,
	}
}

// tokenClaims is the payload of an auth token.
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

// ParseToken validates a JWT token and returns the user ID.
func (m *Manager) ParseToken(tokenString string) (string, error) {
	claims, err := m.Parse(tokenString)
	if err != nil {
		return "", err
	}

	return claims.UserID, nil
}

// Parse validates a JWT token and returns its claims.
func (m *Manager) Parse(tokenString string) (Claims, error) {
	var parsed tokenClaims

	token, err := jwtlib.ParseWithClaims(tokenString, &parsed, func(token *jwtlib.Token) (any, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}

		return []byte(m.secret), nil
	})
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
