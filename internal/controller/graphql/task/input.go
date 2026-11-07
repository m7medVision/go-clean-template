package task

// deref returns the description a mutation was given, or "" when it was left out.
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
