package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/user"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

const _loginURL = "https://example.com/login"

var errInternalServErr = errors.New("internal server error")

func newUserUseCase(t *testing.T) (usecase.User, *MockUserRepo, *MockQueueRepo) {
	t.Helper()

	ctrl := gomock.NewController(t)

	repo := NewMockUserRepo(ctrl)
	queue := NewMockQueueRepo(ctrl)
	jwtManager := jwt.New("test-secret", time.Hour)
	useCase := user.New(repo, queue, jwtManager, _loginURL)

	return useCase, repo, queue
}

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("register success", func(t *testing.T) {
		t.Parallel()

		uc, repo, queue := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
		queue.EXPECT().Send(gomock.Any(), entity.EmailQueue, gomock.Any()).Return(int64(1), nil)

		u, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.NoError(t, err)
		assert.NotEmpty(t, u.ID)
		assert.Equal(t, "testuser", u.Username)
		assert.Equal(t, "test@example.com", u.Email)
	})

	t.Run("register duplicate enqueues no email", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(entity.ErrUserAlreadyExists)

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrUserAlreadyExists)
	})
}

func TestRegister_WelcomeEmail(t *testing.T) {
	t.Parallel()

	t.Run("register enqueues the welcome email", func(t *testing.T) {
		t.Parallel()

		uc, repo, queue := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)

		var body json.RawMessage

		queue.EXPECT().Send(gomock.Any(), entity.EmailQueue, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, b json.RawMessage) (int64, error) {
				body = b

				return 1, nil
			})

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")
		require.NoError(t, err)

		email, err := entity.DecodeEmail(body)
		require.NoError(t, err)
		assert.Equal(t, entity.WelcomeEmail{
			To:       "test@example.com",
			Username: "testuser",
			LoginURL: _loginURL,
		}, email)
	})

	// The user is stored by then; the error still surfaces, since swallowing it would lose the email
	// without a trace. Storing and enqueueing in one transaction is the real fix, not done here.
	t.Run("register returns the enqueue error", func(t *testing.T) {
		t.Parallel()

		uc, repo, queue := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
		queue.EXPECT().Send(gomock.Any(), entity.EmailQueue, gomock.Any()).Return(int64(0), errInternalServErr)

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.ErrorIs(t, err, errInternalServErr)
	})
}

func TestLogin(t *testing.T) {
	t.Parallel()

	t.Run("login success", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		storedUser := entity.User{
			ID: "user-id-123", Username: "testuser",
			Email: "test@example.com", PasswordHash: string(hash),
		}
		repo.EXPECT().GetByEmail(gomock.Any(), "test@example.com").Return(storedUser, nil)

		token, err := uc.Login(context.Background(), "test@example.com", "password123")

		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("login wrong password", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		storedUser := entity.User{
			ID: "user-id-123", Username: "testuser",
			Email: "test@example.com", PasswordHash: string(hash),
		}
		repo.EXPECT().GetByEmail(gomock.Any(), "test@example.com").Return(storedUser, nil)

		token, err := uc.Login(context.Background(), "test@example.com", "wrongpassword")

		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
		assert.Empty(t, token)
	})

	t.Run("login user not found", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		repo.EXPECT().GetByEmail(gomock.Any(), "notfound@example.com").Return(entity.User{}, entity.ErrUserNotFound)

		token, err := uc.Login(context.Background(), "notfound@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
		assert.Empty(t, token)
	})
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	expectedUser := entity.User{
		ID:       "user-id-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	t.Run("get user success", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		repo.EXPECT().GetByID(gomock.Any(), "user-id-123").Return(expectedUser, nil)

		u, err := uc.GetUser(context.Background(), "user-id-123")

		require.NoError(t, err)
		assert.Equal(t, expectedUser, u)
	})

	t.Run("get user not found", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newUserUseCase(t)
		repo.EXPECT().GetByID(gomock.Any(), "missing-id").Return(entity.User{}, entity.ErrUserNotFound)

		_, err := uc.GetUser(context.Background(), "missing-id")

		require.ErrorIs(t, err, entity.ErrUserNotFound)
	})
}

func TestGetUser_GenericError(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUserUseCase(t)

	repo.EXPECT().GetByID(gomock.Any(), "user-id-123").Return(entity.User{}, errInternalServErr)

	_, err := uc.GetUser(context.Background(), "user-id-123")

	require.Error(t, err)
	require.ErrorIs(t, err, errInternalServErr)
}
