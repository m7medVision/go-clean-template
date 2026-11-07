package entity_test

import (
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/require"
)

// A record is its owner's to read. To anyone else a task is forbidden, and a passkey or a conversion
// does not exist.
func TestOnlyTheOwnerReadsARecord(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		readableBy func(userID string) error
		other      error
	}{
		"task":       {(&entity.Task{UserID: _owner}).ReadableBy, entity.ErrTaskForbidden},
		"passkey":    {(&entity.Passkey{UserID: _owner}).ReadableBy, entity.ErrPasskeyNotFound},
		"conversion": {(&entity.Conversion{UserID: _owner}).ReadableBy, entity.ErrConversionNotFound},
	} {
		require.NoError(t, tc.readableBy(_owner), name)
		require.ErrorIs(t, tc.readableBy(_other), tc.other, name)
	}
}

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

// A list's status filter is optional on every transport: an empty one means any status, and any
// other is passed on for the query to validate.
func TestAnEmptyStatusFilterMeansAny(t *testing.T) {
	t.Parallel()

	require.Nil(t, entity.TaskStatusFilter(""))

	filter := entity.TaskStatusFilter(entity.TaskStatusDone)
	require.NotNil(t, filter)
	require.Equal(t, entity.TaskStatusDone, *filter)

	filter = entity.TaskStatusFilter("archived")
	require.NotNil(t, filter)
	require.Equal(t, entity.TaskStatus("archived"), *filter)
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
