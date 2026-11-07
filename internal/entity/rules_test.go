package entity_test

import (
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/require"
)

func TestTaskTextLimits(t *testing.T) {
	t.Parallel()

	for _, ok := range [][2]string{
		{"a", ""},
		{strings.Repeat("é", entity.MaxTaskTitleLen), strings.Repeat("é", entity.MaxTaskDescriptionLen)},
	} {
		require.NoError(t, entity.ValidateTaskText(ok[0], ok[1]))
	}

	require.ErrorIs(t, entity.ValidateTaskText("", ""), entity.ErrInvalidTaskTitle)
	require.ErrorIs(t, entity.ValidateTaskText(strings.Repeat("a", entity.MaxTaskTitleLen+1), ""), entity.ErrInvalidTaskTitle)
	require.ErrorIs(t, entity.ValidateTaskText("a", strings.Repeat("a", entity.MaxTaskDescriptionLen+1)),
		entity.ErrInvalidTaskDescription)
}

func TestTaskStatusMustBeKnown(t *testing.T) {
	t.Parallel()

	require.NoError(t, entity.ValidateTaskStatus(entity.TaskStatusDone))
	require.ErrorIs(t, entity.ValidateTaskStatus("archived"), entity.ErrInvalidTaskStatus)
}

func TestSignUpRules(t *testing.T) {
	t.Parallel()

	require.NoError(t, entity.ValidateSignUp("bob", "bob@example.com", "secret"))

	for name, tc := range map[string]struct {
		username, email, password string
		want                      error
	}{
		"short username":    {"bo", "bob@example.com", "secret", entity.ErrInvalidUsername},
		"long username":     {strings.Repeat("b", entity.MaxUsernameLen+1), "bob@example.com", "secret", entity.ErrInvalidUsername},
		"no email":          {"bob", "", "secret", entity.ErrInvalidEmail},
		"email with a name": {"bob", "Bob <bob@example.com>", "secret", entity.ErrInvalidEmail},
		"not an email":      {"bob", "bob", "secret", entity.ErrInvalidEmail},
		"short password":    {"bob", "bob@example.com", "12345", entity.ErrWeakPassword},
		"empty password":    {"bob", "bob@example.com", "", entity.ErrWeakPassword},
	} {
		require.ErrorIs(t, entity.ValidateSignUp(tc.username, tc.email, tc.password), tc.want, name)
	}
}
