package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail/command"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errRelay = errors.New("relay down")

// noteEmail is a test-only email, so the mail tests do not depend on a real email type.
type noteEmail struct {
	To   string
	Body string
}

func (noteEmail) Template() string    { return "note" }
func (e noteEmail) Recipient() string { return e.To }

func newHandler(t *testing.T) (command.DeliverEmailHandler, *repomock.MockMailRenderer, *repomock.MockMailer) {
	t.Helper()

	ctrl := gomock.NewController(t)
	renderer, mailer := repomock.NewMockMailRenderer(ctrl), repomock.NewMockMailer(ctrl)

	return command.DeliverEmailHandler{Renderer: renderer, Mailer: mailer}, renderer, mailer
}

func note() noteEmail { return noteEmail{To: "john@example.com", Body: "Hi"} }

func rendered() repo.RenderedEmail {
	return repo.RenderedEmail{To: "john@example.com", Subject: "Note", HTML: "<p>Hi</p>\n", Text: "Hi\n"}
}

// The worker drops a message whose email no template exists for, so that error must stay
// recognizable; the mailer expects no call.
func TestARenderErrorSendsNothing(t *testing.T) {
	t.Parallel()

	h, renderer, _ := newHandler(t)
	renderer.EXPECT().Render(gomock.Any(), note()).Return(repo.RenderedEmail{}, entity.ErrUnknownEmail)

	_, err := h.Handle(context.Background(), command.DeliverEmail{Email: note()})

	require.ErrorIs(t, err, entity.ErrUnknownEmail)
}

func TestASendErrorIsReturnedForTheWorkerToRetry(t *testing.T) {
	t.Parallel()

	h, renderer, mailer := newHandler(t)
	renderer.EXPECT().Render(gomock.Any(), note()).Return(rendered(), nil)
	mailer.EXPECT().Send(gomock.Any(), rendered()).Return(errRelay)

	_, err := h.Handle(context.Background(), command.DeliverEmail{Email: note()})

	require.ErrorIs(t, err, errRelay)
}
