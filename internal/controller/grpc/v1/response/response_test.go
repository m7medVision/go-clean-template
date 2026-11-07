package response_test

import (
	"math"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/stretchr/testify/assert"
)

func TestTime(t *testing.T) {
	t.Parallel()

	const fourHours = 4 * 60 * 60

	gulf := time.FixedZone("UTC+4", fourHours)

	tests := map[string]struct {
		in   time.Time
		want string
	}{
		"a UTC time":                    {time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC), "2026-03-15T12:30:00Z"},
		"another zone, written in UTC":  {time.Date(2026, 3, 15, 16, 30, 0, 0, gulf), "2026-03-15T12:30:00Z"},
		"fractions of a second dropped": {time.Date(2026, 3, 15, 12, 30, 0, 999, time.UTC), "2026-03-15T12:30:00Z"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, response.Time(tt.in))
		})
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   int
		want int32
	}{
		"in range":     {42, 42},
		"negative":     {-1, 0},
		"above int32":  {math.MaxInt32 + 1, math.MaxInt32},
		"int32's edge": {math.MaxInt32, math.MaxInt32},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, response.Count(tt.in))
		})
	}
}
