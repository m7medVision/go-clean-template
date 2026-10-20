package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSMTPDown = errors.New("smtp down")

// fakeMail records what the handler delivers and fails with err.
type fakeMail struct {
	err       error
	delivered []entity.Email
}

func (f *fakeMail) Deliver(_ context.Context, email entity.Email) error {
	f.delivered = append(f.delivered, email)

	return f.err
}

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

func TestMailController_Handle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          func(t *testing.T) json.RawMessage
		deliverErr    error
		wantErr       error
		wantDelivered int
	}{
		{
			name:          "delivers the decoded email and acknowledges it",
			body:          welcomeBody,
			wantDelivered: 1,
		},
		{
			name:          "a body that is not an email is dropped, retrying cannot fix it",
			body:          func(*testing.T) json.RawMessage { return json.RawMessage(`"not an object"`) },
			wantDelivered: 0,
		},
		{
			name: "an email this build does not know is dropped",
			body: func(*testing.T) json.RawMessage {
				return json.RawMessage(`{"template":"nope","data":{}}`)
			},
			wantDelivered: 0,
		},
		{
			name:          "an email without a template is dropped",
			body:          welcomeBody,
			deliverErr:    fmt.Errorf("render: %w", entity.ErrUnknownEmail),
			wantDelivered: 1,
		},
		{
			name:          "a failed send is returned so the message is redelivered",
			body:          welcomeBody,
			deliverErr:    errSMTPDown,
			wantErr:       errSMTPDown,
			wantDelivered: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mail := &fakeMail{err: tc.deliverErr}
			c := v1.NewMailController(mail, logger.New("error"))

			err := c.Handle(context.Background(), &worker.Message{ID: 1, Body: tc.body(t)})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, mail.delivered, tc.wantDelivered)
		})
	}
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
