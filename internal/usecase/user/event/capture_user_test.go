package event_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCaptureUserSendsARegistrationWithItsMethod(t *testing.T) {
	t.Parallel()

	at := time.Unix(1, 0).UTC()

	tests := []struct {
		name  string
		event entity.Event
		want  analytics.SignUpMethod
	}{
		{name: "password", event: entity.UserRegistered{UserID: "u1", Email: "jo@example.com"}, want: analytics.SignUpPassword},
		{name: "oidc", event: entity.UserRegisteredViaOIDC{UserID: "u1", Email: "jo@example.com"}, want: analytics.SignUpOIDC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := repomock.NewMockAnalytics(gomock.NewController(t))
			a.EXPECT().Capture(gomock.Any(), &analytics.Event{
				ID: "user-u1-1", DistinctID: "u1", At: at, Properties: analytics.UserRegistered{Method: tt.want},
			}).Return(nil)

			require.NoError(t, event.CaptureUser{Analytics: a}.Handle(t.Context(),
				&entity.RecordedEvent{StreamID: "user-u1", StreamVersion: 1, RecordedAt: at, Event: tt.event}))
		})
	}
}

func TestCaptureUserStartsAtHeadAndSkipsImports(t *testing.T) {
	t.Parallel()

	h := event.CaptureUser{}
	assert.True(t, h.StartsAtHead())
	assert.NotContains(t, h.Handles(), entity.Event(entity.UserImported{}))
}

func TestCaptureUserReturnsAFailureForTheRunnerToRetry(t *testing.T) {
	t.Parallel()

	a := repomock.NewMockAnalytics(gomock.NewController(t))
	a.EXPECT().Capture(gomock.Any(), gomock.Any()).Return(errQueue)

	require.ErrorIs(t, event.CaptureUser{Analytics: a}.Handle(t.Context(),
		&entity.RecordedEvent{StreamID: "user-u1", Event: entity.UserRegistered{UserID: "u1"}}), errQueue)
}
