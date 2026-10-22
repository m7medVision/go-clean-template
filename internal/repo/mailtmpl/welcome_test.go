package mailtmpl_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_WelcomeContent(t *testing.T) {
	t.Parallel()

	r, err := mailtmpl.New()
	require.NoError(t, err)

	msg, err := r.Render(context.Background(), entity.WelcomeEmail{
		To:       "john@example.com",
		Username: "<b>johndoe</b>",
		LoginURL: "https://example.com/login",
	})
	require.NoError(t, err)

	assert.Contains(t, msg.Subject, "<b>johndoe</b>", "the subject is plain text, not HTML")
	assert.Contains(t, msg.HTML, "&lt;b&gt;johndoe&lt;/b&gt;", "the HTML escapes user data")
	assert.NotContains(t, msg.HTML, "<b>johndoe</b>")
	assert.Contains(t, msg.HTML, `href="https://example.com/login"`)
	assert.Contains(t, msg.Text, "<b>johndoe</b>", "the text part is not HTML-escaped")
	assert.Contains(t, msg.Text, "https://example.com/login")
}
