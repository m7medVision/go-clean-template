package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/user"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newWelcomingUserUseCase is newUserUseCase with the notifier exposed, so a test sets its own
// expectations on Welcome.
func newWelcomingUserUseCase(t *testing.T) (usecase.User, *MockUserRepo, *MockUserNotifier) {
	t.Helper()

	ctrl := gomock.NewController(t)

	repo := NewMockUserRepo(ctrl)
	notifier := NewMockUserNotifier(ctrl)

	return user.New(repo, notifier, jwt.New("test-secret", time.Hour)), repo, notifier
}

func TestRegister_Welcome(t *testing.T) {
	t.Parallel()

	t.Run("register welcomes the stored user", func(t *testing.T) {
		t.Parallel()

		uc, repo, notifier := newWelcomingUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)

		var welcomed entity.User

		notifier.EXPECT().Welcome(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, u *entity.User) error {
				welcomed = *u

				return nil
			})

		u, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")
		require.NoError(t, err)

		assert.Equal(t, u, welcomed)
	})

	t.Run("register duplicate sends no welcome", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newWelcomingUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(entity.ErrUserAlreadyExists)

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrUserAlreadyExists)
	})

	// The user is stored by then; the error still surfaces, since swallowing it would lose the welcome
	// without a trace. Storing and enqueueing in one transaction is the real fix, not done here.
	t.Run("register returns the welcome error", func(t *testing.T) {
		t.Parallel()

		uc, repo, notifier := newWelcomingUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
		notifier.EXPECT().Welcome(gomock.Any(), gomock.Any()).Return(errInternalServErr)

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.ErrorIs(t, err, errInternalServErr)
	})
}
