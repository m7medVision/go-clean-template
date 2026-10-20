package usecase_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newMailUseCase(t *testing.T) (usecase.Mail, *MockMailRenderer, *MockMailer) {
	t.Helper()

	ctrl := gomock.NewController(t)

	renderer := NewMockMailRenderer(ctrl)
	mailer := NewMockMailer(ctrl)

	return mail.New(renderer, mailer), renderer, mailer
}

func TestDeliver(t *testing.T) {
	t.Parallel()

	welcome := entity.WelcomeEmail{To: "john@example.com", Username: "johndoe", LoginURL: "https://example.com/login"}
	rendered := smtp.Message{To: "john@example.com", Subject: "Welcome", HTML: "<p>Hi</p>\n", Text: "Hi\n"}

	t.Run("sends the rendered message", func(t *testing.T) {
		t.Parallel()

		uc, renderer, mailer := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), welcome).Return(rendered, nil)
		mailer.EXPECT().Send(gomock.Any(), rendered).Return(nil)

		err := uc.Deliver(context.Background(), welcome)

		require.NoError(t, err)
	})

	t.Run("render error sends nothing", func(t *testing.T) {
		t.Parallel()

		uc, renderer, _ := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), welcome).Return(smtp.Message{}, errInternalServErr)

		err := uc.Deliver(context.Background(), welcome)

		require.ErrorIs(t, err, errInternalServErr)
	})

	t.Run("unknown email stays recognizable", func(t *testing.T) {
		t.Parallel()

		uc, renderer, _ := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), welcome).Return(smtp.Message{}, entity.ErrUnknownEmail)

		err := uc.Deliver(context.Background(), welcome)

		require.ErrorIs(t, err, entity.ErrUnknownEmail)
	})

	t.Run("send error", func(t *testing.T) {
		t.Parallel()

		uc, renderer, mailer := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), welcome).Return(rendered, nil)
		mailer.EXPECT().Send(gomock.Any(), rendered).Return(errInternalServErr)

		err := uc.Deliver(context.Background(), welcome)

		require.ErrorIs(t, err, errInternalServErr)
	})
}
