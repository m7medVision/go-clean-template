package event_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errQueue = errors.New("queue down")

func handle(t *testing.T, notifier *repomock.MockUserNotifier, e entity.Event) error {
	t.Helper()

	return event.SendWelcomeEmail{Notifier: notifier}.Handle(context.Background(),
		&entity.RecordedEvent{Position: 1, StreamID: "user-u1", Event: e})
}

func TestSendWelcomeEmailWelcomesAUserWhoRegistered(t *testing.T) {
	t.Parallel()

	notifier := repomock.NewMockUserNotifier(gomock.NewController(t))
	notifier.EXPECT().Welcome(gomock.Any(), &entity.User{ID: "u1", Username: "jo", Email: "jo@example.com"}).Return(nil)

	require.NoError(t, handle(t, notifier, entity.UserRegistered{UserID: "u1", Username: "jo", Email: "jo@example.com"}))
}

func TestSendWelcomeEmailLeavesImportsAndSSOSignInsAlone(t *testing.T) {
	t.Parallel()

	notifier := repomock.NewMockUserNotifier(gomock.NewController(t))

	require.NoError(t, handle(t, notifier, entity.UserImported{UserID: "u1"}))
	require.NoError(t, handle(t, notifier, entity.UserRegisteredViaOIDC{UserID: "u1"}))
	require.NoError(t, handle(t, notifier, entity.TaskDeleted{TaskID: "t1"}))
}

func TestSendWelcomeEmailReturnsAFailureForTheRunnerToRetry(t *testing.T) {
	t.Parallel()

	notifier := repomock.NewMockUserNotifier(gomock.NewController(t))
	notifier.EXPECT().Welcome(gomock.Any(), gomock.Any()).Return(errQueue)

	require.ErrorIs(t, handle(t, notifier, entity.UserRegistered{UserID: "u1"}), errQueue)
}

func TestSendWelcomeEmailIsNamed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "send-welcome-email", event.SendWelcomeEmail{}.Name())
}
