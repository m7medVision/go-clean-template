// Package response writes the values every service's responses carry into their proto field types.
package response

import (
	"math"
	"time"
)

// Time is t as every response writes a timestamp: RFC 3339 in UTC, to the second.
func Time(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// Count is n as a proto int32, clamped to its range.
func Count(n int) int32 {
	if n < 0 {
		return 0
	}

	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}
