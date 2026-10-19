package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

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

// findMailTo returns the ID of the newest message to addr, or "" while there is none.
func findMailTo(t *testing.T, addr string) string {
	t.Helper()

	found := mailpitGet[struct {
		Messages []mailpitMessage `json:"messages"`
	}](t, "/search?query="+url.QueryEscape(`to:"`+addr+`"`))

	if len(found.Messages) == 0 {
		return ""
	}

	return found.Messages[0].ID
}

// Registering over REST enqueues a welcome email; the worker renders it and sends it to Mailpit.
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
}
