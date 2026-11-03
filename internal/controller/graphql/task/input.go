package task

import "unicode/utf8"

// The same limits as the REST request DTOs (restapi/v1/request/task.go) and the web client's Zod
// schema.
const (
	_maxTitleLen       = 255
	_maxDescriptionLen = 1000
)

// validInput reports whether a task's title and description fit their limits.
func validInput(title string, description *string) bool {
	titleLen := utf8.RuneCountInString(title)

	return titleLen > 0 && titleLen <= _maxTitleLen &&
		(description == nil || utf8.RuneCountInString(*description) <= _maxDescriptionLen)
}

// deref returns the description a mutation was given, or "" when it was left out.
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
