package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
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

func TestMailController_HandleDeliversWhatWasEnqueued(t *testing.T) {
	t.Parallel()

	mail := &fakeMail{}
	c := v1.NewMailController(mail, logger.New("error"))

	require.NoError(t, c.Handle(context.Background(), &worker.Message{ID: 1, Body: welcomeBody(t)}))

	require.Len(t, mail.delivered, 1)
	assert.Equal(t, entity.WelcomeEmail{
		To:       "john@example.com",
		Username: "johndoe",
		LoginURL: "https://example.com/login",
	}, mail.delivered[0])
}
