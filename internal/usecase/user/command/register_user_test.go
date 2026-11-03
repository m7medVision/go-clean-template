package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func now() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 999, time.UTC) }

func TestRegisterUserRecordsUserRegisteredWithAHash(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	h := command.RegisterUserHandler{Store: store, Now: now}

	written, err := h.Handle(context.Background(),
		command.RegisterUser{Username: "jo", Email: "jo@example.com", Password: "secret-pass"})
	require.NoError(t, err)

	events := store.Events(entity.UserStreamID(written.ID))
	require.Len(t, events, 1)

	registered, ok := events[0].(entity.UserRegistered)
	require.True(t, ok)
	assert.Equal(t, written.ID, registered.UserID)
	assert.Equal(t, "jo", registered.Username)
	assert.Equal(t, "jo@example.com", registered.Email)
	assert.Equal(t, now().Truncate(time.Microsecond), registered.At)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(registered.PasswordHash), []byte("secret-pass")))
}

func TestRegisterUserRefusesATakenEmailOrUsername(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	h := command.RegisterUserHandler{Store: store}

	_, err := h.Handle(context.Background(), command.RegisterUser{Username: "jo", Email: "jo@example.com", Password: "p"})
	require.NoError(t, err)

	_, err = h.Handle(context.Background(), command.RegisterUser{Username: "other", Email: "jo@example.com", Password: "p"})
	require.ErrorIs(t, err, entity.ErrUserAlreadyExists)

	_, err = h.Handle(context.Background(), command.RegisterUser{Username: "jo", Email: "other@example.com", Password: "p"})
	require.ErrorIs(t, err, entity.ErrUserAlreadyExists)
}

func TestRegisterUserIsNamed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "RegisterUser", command.RegisterUser{}.CommandName())
}
