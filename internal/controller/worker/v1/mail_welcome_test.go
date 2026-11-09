package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSMTPDown = errors.New("smtp down")

func welcomeBody(t *testing.T) json.RawMessage {
	t.Helper()

	body, err := entity.EncodeEmail(entity.WelcomeEmail{
		To:       "john@example.com",
		Username: "johndoe",
		LoginURL: "https://example.com/login",
	})
	require.NoError(t, err)

	return body
}

func TestMailController_HandleDeliversWhatWasPublished(t *testing.T) {
	t.Parallel()

	mail := &fakeMail{}
	c := v1.NewMailController(mail.commands(), slog.New(slog.DiscardHandler))

	require.NoError(t, c.Handle(context.Background(), welcomeBody(t)))

	require.Len(t, mail.delivered, 1)
	assert.Equal(t, entity.WelcomeEmail{
		To:       "john@example.com",
		Username: "johndoe",
		LoginURL: "https://example.com/login",
	}, mail.delivered[0])
}
