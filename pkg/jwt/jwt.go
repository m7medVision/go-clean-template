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

// GenerateToken creates a new JWT token for the given user ID.
func (m *Manager) GenerateToken(userID string) (string, error) {
	now := time.Now()

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(m.duration)),
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
	token, err := jwtlib.Parse(tokenString, func(token *jwtlib.Token) (any, error) {
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

	claims := Claims{UserID: sub}

	issuedAt, err := token.Claims.GetIssuedAt()
	if err != nil {
		return Claims{}, fmt.Errorf("jwt - Parse - GetIssuedAt: %w", err)
	}

	if issuedAt != nil {
		claims.IssuedAt = issuedAt.Time
	}

	return claims, nil
}
