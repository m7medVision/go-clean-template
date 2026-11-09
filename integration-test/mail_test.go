package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// mailpitURL is Mailpit's API: the compose service, or the CI runner via /etc/hosts.
	mailpitURL = "http://mailpit:8025/api/v1"

	mailWait = 30 * time.Second
	mailPoll = 500 * time.Millisecond
)

type mailpitMessage struct {
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
	Text    string `json:"Text"`
	HTML    string `json:"HTML"`
}

// mailpitGet decodes one Mailpit API response into T.
func mailpitGet[T any](t *testing.T, path string) T {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doWebRequestWithTimeout(ctx, http.MethodGet, mailpitURL+path, http.NoBody)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s", path)

	return parseJSON[T](t, resp)
}

// mailsTo lists the messages Mailpit holds for addr, newest first.
func mailsTo(t *testing.T, addr string) []mailpitMessage {
	t.Helper()

	return mailpitGet[struct {
		Messages []mailpitMessage `json:"messages"`
	}](t, "/search?query="+url.QueryEscape(`to:"`+addr+`"`)).Messages
}

// findMailTo returns the ID of the newest message to addr, or "" while there is none.
func findMailTo(t *testing.T, addr string) string {
	t.Helper()

	found := mailsTo(t, addr)
	if len(found) == 0 {
		return ""
	}

	return found[0].ID
}

// requireNoWelcomeToFollow waits until no second welcome email to the user can still arrive: the
// event handler's checkpoint has passed the user's registration, so it will not publish for it
// again, and the email it published has reached Mailpit.
func requireNoWelcomeToFollow(t *testing.T, userID, email string) {
	t.Helper()

	ctx := t.Context()
	db := newPostgres(t).Executor(ctx)

	var registered int64

	err := db.QueryRow(ctx, "SELECT max(position) FROM events WHERE stream_id = $1", entity.UserStreamID(userID)).
		Scan(&registered)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		var at int64

		err := db.QueryRow(ctx, "SELECT position FROM projection_checkpoints WHERE name = $1",
			event.SendWelcomeEmail{}.Name()).Scan(&at)

		return err == nil && at >= registered
	}, mailWait, mailPoll, "the welcome email handler never passed position %d", registered)

	require.Eventually(t, func() bool { return findMailTo(t, email) != "" },
		mailWait, mailPoll, "the mail topic never delivered the email to %s", email)
}

// Registering over REST sends a welcome email, once: the event handler publishes it to the mail
// topic under the user's message ID, so a republish after a crash is dropped by JetStream. The
// worker renders it and sends it to Mailpit.
func TestHTTPRegisterSendsWelcomeEmail(t *testing.T) {
	name := sanitizeTestName(t)
	email := name + "@test.com"

	resp := registerUser(t, name, email, testPassword)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var id string

	require.Eventually(t, func() bool {
		id = findMailTo(t, email)

		return id != ""
	}, mailWait, mailPoll, "no welcome email reached Mailpit for %s", email)

	msg := mailpitGet[mailpitMessage](t, "/message/"+id)

	assert.Equal(t, "Welcome to Go Clean Template, "+name, msg.Subject)
	assert.Contains(t, msg.Text, "Hi "+name+",")
	assert.Contains(t, msg.HTML, "Hi "+name+",")
	assert.Contains(t, msg.Text, "/login")
	assert.Contains(t, msg.HTML, `href="http://localhost/login"`)

	requireNoWelcomeToFollow(t, profileID(t, loginUser(t, email, testPassword)), email)
	assert.Len(t, mailsTo(t, email), 1, "a registration sends one welcome email")
}
