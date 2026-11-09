package analytics_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventNamesAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, p := range analytics.Events() {
		assert.False(t, seen[p.Name()], p.Name())
		seen[p.Name()] = true
	}
}

func TestEncodeDecodeRoundTripsEveryEvent(t *testing.T) {
	t.Parallel()

	samples := []analytics.Properties{
		analytics.UserRegistered{Method: analytics.SignUpOIDC},
		analytics.TaskCreated{},
		analytics.TaskStatusChanged{From: entity.TaskStatusTodo, To: entity.TaskStatusDone},
		analytics.CurrencyConverted{From: "USD", To: "EUR"},
	}
	require.Len(t, samples, len(analytics.Events()))

	for _, p := range samples {
		t.Run(p.Name(), func(t *testing.T) {
			t.Parallel()

			in := &analytics.Event{ID: "task-1-1", DistinctID: "u1", At: time.Unix(1, 0).UTC(), Properties: p}

			body, err := analytics.Encode(in)
			require.NoError(t, err)

			out, err := analytics.Decode(body)
			require.NoError(t, err)
			assert.Equal(t, in, out)
		})
	}
}

func TestDecodeRefusesAnUnknownEvent(t *testing.T) {
	t.Parallel()

	_, err := analytics.Decode([]byte(`{"name":"nope","properties":{}}`))
	require.ErrorIs(t, err, analytics.ErrUnknownEvent)
}

func TestNoPropertyIsAFreeString(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{"CurrencyConverted.From": true, "CurrencyConverted.To": true}

	for _, p := range analytics.Events() {
		typ := reflect.TypeOf(p)
		for f := range typ.Fields() {
			if f.Type.Kind() == reflect.String && f.Type.PkgPath() == "" {
				assert.True(t, allowed[typ.Name()+"."+f.Name], "%s.%s is a free string", typ.Name(), f.Name)
			}
		}
	}
}

func TestEventIDIsTheStreamAndVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "task-7-3", analytics.EventID(&entity.RecordedEvent{StreamID: "task-7", StreamVersion: 3}))
}
