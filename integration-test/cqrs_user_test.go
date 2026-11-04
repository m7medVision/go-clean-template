package integration_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// duplicateMailWait is how long the test watches for a second welcome email after the first.
const duplicateMailWait = 3 * time.Second

type profileResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// A user who registers can sign in and read their profile at once: the write does not answer
// before the user read model has them.
func TestARegisteredUserCanSignInAndReadTheirProfileAtOnce(t *testing.T) {
	name := sanitizeTestName(t)

	resp := registerUser(t, name, name+"@test.com", testPassword)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	token := loginUser(t, name+"@test.com", testPassword)

	profile := httpGetJSON[profileResponse](t, token, "/user/profile")
	assert.Equal(t, name, profile.Username)
}

// The welcome email is enqueued by an event handler inside the transaction that moves its
// checkpoint, so a registration sends it once, never twice.
func TestTheWelcomeEmailIsSentExactlyOnce(t *testing.T) {
	name := sanitizeTestName(t)
	email := name + "@test.com"

	resp := registerUser(t, name, email, testPassword)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	countMail := func() int {
		found := mailpitGet[struct {
			Messages []mailpitMessage `json:"messages"`
		}](t, "/search?query="+url.QueryEscape(`to:"`+email+`"`))

		return len(found.Messages)
	}

	require.Eventually(t, func() bool { return countMail() > 0 }, mailWait, mailPoll)

	time.Sleep(duplicateMailWait)
	assert.Equal(t, 1, countMail())
}
