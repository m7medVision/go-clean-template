package entity

import (
	"fmt"
	"net/mail"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
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

// User -.
type User struct {
	ID           string    `json:"id"         example:"550e8400-e29b-41d4-a716-446655440000"`
	Username     string    `json:"username"    example:"johndoe"`
	Email        string    `json:"email"       example:"john@example.com"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"  example:"2026-01-01T00:00:00Z"`
	UpdatedAt    time.Time `json:"updated_at"  example:"2026-01-01T00:00:00Z"`
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
