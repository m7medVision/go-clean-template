package mailer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/mailer"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRelay = errors.New("relay down")

type fakeClient struct {
	sent []smtp.Message
	err  error
}

func (f *fakeClient) Send(_ context.Context, m smtp.Message) error {
	f.sent = append(f.sent, m)

	return f.err
}

func TestSendHandsTheRenderedEmailToTheClient(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	email := repo.RenderedEmail{To: "john@example.com", Subject: "Welcome", HTML: "<p>Hi</p>", Text: "Hi"}

	require.NoError(t, mailer.New(client).Send(context.Background(), email))

	assert.Equal(t, []smtp.Message{{To: "john@example.com", Subject: "Welcome", HTML: "<p>Hi</p>", Text: "Hi"}}, client.sent)
}

func TestSendReturnsTheClientsError(t *testing.T) {
	t.Parallel()

	err := mailer.New(&fakeClient{err: errRelay}).Send(context.Background(), repo.RenderedEmail{To: "john@example.com"})

	require.ErrorIs(t, err, errRelay)
}
