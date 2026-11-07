package entity

import (
	"fmt"
	"net/mail"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = newError(KindNotFound, "user not found")
	ErrUserAlreadyExists = newError(KindExists, "user already exists")
	// ErrOIDCIdentityTaken is a first SSO sign-in of an identity another sign-in has just created a
	// user for: the race's loser signs in to the winner's user.
	ErrOIDCIdentityTaken  = newError(KindExists, "user already exists")
	ErrInvalidCredentials = newError(KindUnauthenticated, "invalid credentials")
	ErrInvalidUsername    = newError(KindInvalid, fmt.Sprintf("username must be %d–%d characters", MinUsernameLen, MaxUsernameLen))
	ErrInvalidEmail       = newError(KindInvalid, "email must be an address such as name@example.com")
	ErrWeakPassword       = newError(KindInvalid, fmt.Sprintf("password must be at least %d characters", MinPasswordLen))
	// ErrOIDCDisabled is an SSO request to an app with no OpenID Connect provider configured.
	ErrOIDCDisabled = newError(KindNotFound, "sso sign-in is not configured")
	// ErrOIDCUnavailable is an SSO sign-in while the OpenID Connect provider cannot be reached.
	ErrOIDCUnavailable = newError(KindUpstream, "sso provider unavailable")
	// ErrOIDCCeremonyInvalid is an SSO sign-in that cannot complete: a bad, expired or replayed
	// state token, code or ID token, or an ID token without an email.
	ErrOIDCCeremonyInvalid = newError(KindUnauthenticated, "invalid sso sign-in")
	// ErrAccountConflict is an SSO sign-in whose email or username already belongs to another account.
	// Accounts are never linked by email, so the sign-in is refused.
	ErrAccountConflict = newError(KindExists,
		"an account with this email or username already exists; sign in with it instead")
)

const (
	// MinUsernameLen and MaxUsernameLen bound a username a user signs up with, in characters.
	MinUsernameLen = 3
	MaxUsernameLen = 255
	// MinPasswordLen is the shortest password a user may sign up with, in characters.
	MinPasswordLen = 6
)

// ValidateSignUp refuses a username outside MinUsernameLen–MaxUsernameLen, an email that is not a
// bare address, and a password shorter than MinPasswordLen.
func ValidateSignUp(username, email, password string) error {
	if n := utf8.RuneCountInString(username); n < MinUsernameLen || n > MaxUsernameLen {
		return ErrInvalidUsername
	}

	if addr, err := mail.ParseAddress(email); err != nil || addr.Name != "" || addr.Address != email {
		return ErrInvalidEmail
	}

	if utf8.RuneCountInString(password) < MinPasswordLen {
		return ErrWeakPassword
	}

	return nil
}

type User struct {
	ID           string    `json:"id"         example:"550e8400-e29b-41d4-a716-446655440000"`
	Username     string    `json:"username"    example:"johndoe"`
	Email        string    `json:"email"       example:"john@example.com"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"  example:"2026-01-01T00:00:00Z"`
	UpdatedAt    time.Time `json:"updated_at"  example:"2026-01-01T00:00:00Z"`
}

// SSOIdentity is who signed in at an OpenID Connect provider: the identity (issuer and subject),
// the email and username a first sign-in creates the user with, and the app roles the provider's
// claims map to.
type SSOIdentity struct {
	Issuer   string
	Subject  string
	Email    string
	Username string
	Roles    []Role
}

// HashPassword is the bcrypt hash a user's password is stored as.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt.GenerateFromPassword: %w", err)
	}

	return string(hash), nil
}

// PasswordMatches reports whether password is u's. A user with no password, created by an SSO
// sign-in, has none to match.
func (u *User) PasswordMatches(password string) bool {
	return u.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}
