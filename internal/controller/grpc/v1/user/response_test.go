package user_test

import (
	"testing"
	"time"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcuser "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/user"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userResponseGetter interface {
	GetId() string
	GetUsername() string
	GetEmail() string
	GetCreatedAt() string
	GetUpdatedAt() string
}

func TestNewUserResponse(t *testing.T) {
	t.Parallel()

	user := &entity.User{
		ID:        "user-id-456",
		Username:  "anotheruser",
		Email:     "another@example.com",
		CreatedAt: time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
	}

	for name, got := range map[string]userResponseGetter{
		"RegisterResponse":   grpcuser.NewUserResponse[v1.RegisterResponse](user),
		"GetProfileResponse": grpcuser.NewUserResponse[v1.GetProfileResponse](user),
	} {
		require.NotNil(t, got, name)
		assert.Equal(t, user.ID, got.GetId(), name)
		assert.Equal(t, user.Username, got.GetUsername(), name)
		assert.Equal(t, user.Email, got.GetEmail(), name)
		assert.Equal(t, "2026-03-15T12:30:00Z", got.GetCreatedAt(), name)
		assert.Equal(t, "2026-03-16T08:00:00Z", got.GetUpdatedAt(), name)
	}
}
