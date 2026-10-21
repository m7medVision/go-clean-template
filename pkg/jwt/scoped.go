package jwt

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// _scopedKeyLen is the HS256 key size derived for each purpose.
const _scopedKeyLen = 32

// Scoped signs short-lived opaque payloads for one purpose. Its key is derived from the shared
// secret with HKDF, using the purpose as the info label, and the purpose is also the token audience,
// so a Scoped token is never accepted by Manager, by a Scoped of another purpose, or the reverse.
type Scoped struct {
	key      []byte
	purpose  string
	duration time.Duration
}

type scopedClaims struct {
	Payload []byte `json:"pld"`
	jwtlib.RegisteredClaims
}

// NewScoped -.
func NewScoped(secret, purpose string, duration time.Duration) (*Scoped, error) {
	key, err := hkdf.Key(sha256.New, []byte(secret), nil, purpose, _scopedKeyLen)
	if err != nil {
		return nil, fmt.Errorf("jwt - NewScoped - hkdf.Key: %w", err)
	}

	return &Scoped{
		key:      key,
		purpose:  purpose,
		duration: duration,
	}, nil
}

// Sign returns a token carrying payload that expires after the configured duration.
func (s *Scoped) Sign(payload []byte) (string, error) {
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, scopedClaims{
		Payload:   payload,
		Audience:  jwtlib.ClaimStrings{s.purpose},
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(s.duration)),
	})

	tokenString, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("jwt - Scoped.Sign - token.SignedString: %w", err)
	}

	return tokenString, nil
}

// Verify checks the signature, audience and expiry, and returns the payload.
func (s *Scoped) Verify(tokenString string) ([]byte, error) {
	var claims scopedClaims

	_, err := jwtlib.ParseWithClaims(tokenString, &claims, func(*jwtlib.Token) (any, error) {
		return s.key, nil
	},
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
		jwtlib.WithAudience(s.purpose),
		jwtlib.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("jwt - Scoped.Verify - jwtlib.ParseWithClaims: %w", err)
	}

	return claims.Payload, nil
}
