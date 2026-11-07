package query_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func users(t *testing.T) *repomock.MockUserReadModel {
	t.Helper()

	return repomock.NewMockUserReadModel(gomock.NewController(t))
}

func hashed(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	return string(hash)
}

func TestLogInIssuesATokenWithTheLocalRoles(t *testing.T) {
	t.Parallel()

	read := users(t)
	read.EXPECT().ByEmail(gomock.Any(), "jo@example.com").
		Return(entity.User{ID: "u1", PasswordHash: hashed(t, "secret")}, nil)

	tokens := jwt.New("key", time.Hour)
	signedIn, err := query.LogInHandler{Users: read, Sessions: session.New(tokens)}.Handle(context.Background(),
		query.LogIn{Email: "jo@example.com", Password: "secret"})
	require.NoError(t, err)
	assert.Equal(t, "u1", signedIn.ID)

	claims, err := tokens.Parse(signedIn.Token)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.UserID)
	assert.Equal(t, []string{"user"}, claims.Roles, "a password sign-in grants only the user role")
}

func TestLogInRefusesEveryWrongCredentialAlike(t *testing.T) {
	t.Parallel()

	for name, found := range map[string]struct {
		user entity.User
		err  error
	}{
		"unknown email":   {err: entity.ErrUserNotFound},
		"wrong password":  {user: entity.User{ID: "u1", PasswordHash: hashed(t, "other")}},
		"no password set": {user: entity.User{ID: "u1"}},
	} {
		read := users(t)
		read.EXPECT().ByEmail(gomock.Any(), "jo@example.com").Return(found.user, found.err)

		_, err := query.LogInHandler{Users: read, Sessions: session.New(jwt.New("key", time.Hour))}.Handle(context.Background(),
			query.LogIn{Email: "jo@example.com", Password: "secret"})

		require.ErrorIs(t, err, entity.ErrInvalidCredentials, name)
	}
}

// A read that fails, a database outage, is the app's failure, logged as one: not a wrong credential.
func TestLogInReturnsAReadFailure(t *testing.T) {
	t.Parallel()

	read := users(t)
	read.EXPECT().ByEmail(gomock.Any(), "jo@example.com").Return(entity.User{}, io.ErrUnexpectedEOF)

	_, err := query.LogInHandler{Users: read, Sessions: session.New(jwt.New("key", time.Hour))}.Handle(context.Background(),
		query.LogIn{Email: "jo@example.com", Password: "secret"})

	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.NotErrorIs(t, err, entity.ErrInvalidCredentials)
}
