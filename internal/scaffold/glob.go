package scaffold

import (
	"path"
	"strings"
)

// match reports whether the slash-separated name matches pattern, where `**` matches any number of
// path segments and every other segment is a path.Match pattern.
func match(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			return matchDoubleStar(pattern[1:], name)
		}

		if len(name) == 0 {
			return false
		}

		if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
			return false
		}

		pattern, name = pattern[1:], name[1:]
	}

	return len(name) == 0
}

// matchDoubleStar matches the rest of a pattern after `**` against every suffix of name.
func matchDoubleStar(rest, name []string) bool {
	for i := 0; i <= len(name); i++ {
		if matchSegments(rest, name[i:]) {
			return true
		}
	}

	return false
}

// matchAny reports whether name matches one of patterns.
func matchAny(patterns []string, name string) bool {
	for _, p := range patterns {
		if match(p, name) {
			return true
		}
	}

	return false
}
