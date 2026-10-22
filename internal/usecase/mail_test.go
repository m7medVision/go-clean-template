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

// noteEmail is a test-only email, so the mail use case's tests do not depend on a real email type.
type noteEmail struct {
	To   string
	Body string
}

func (noteEmail) Template() string    { return "note" }
func (e noteEmail) Recipient() string { return e.To }

func TestDeliver(t *testing.T) {
	t.Parallel()

	note := noteEmail{To: "john@example.com", Body: "Hi"}
	rendered := smtp.Message{To: "john@example.com", Subject: "Note", HTML: "<p>Hi</p>\n", Text: "Hi\n"}

	t.Run("sends the rendered message", func(t *testing.T) {
		t.Parallel()

		uc, renderer, mailer := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), note).Return(rendered, nil)
		mailer.EXPECT().Send(gomock.Any(), rendered).Return(nil)

		err := uc.Deliver(context.Background(), note)

		require.NoError(t, err)
	})

	t.Run("render error sends nothing", func(t *testing.T) {
		t.Parallel()

		uc, renderer, _ := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), note).Return(smtp.Message{}, errInternalServErr)

		err := uc.Deliver(context.Background(), note)

		require.ErrorIs(t, err, errInternalServErr)
	})

	t.Run("unknown email stays recognizable", func(t *testing.T) {
		t.Parallel()

		uc, renderer, _ := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), note).Return(smtp.Message{}, entity.ErrUnknownEmail)

		err := uc.Deliver(context.Background(), note)

		require.ErrorIs(t, err, entity.ErrUnknownEmail)
	})

	t.Run("send error", func(t *testing.T) {
		t.Parallel()

		uc, renderer, mailer := newMailUseCase(t)
		renderer.EXPECT().Render(gomock.Any(), note).Return(rendered, nil)
		mailer.EXPECT().Send(gomock.Any(), rendered).Return(errInternalServErr)

		err := uc.Deliver(context.Background(), note)

		require.ErrorIs(t, err, errInternalServErr)
	})
}
