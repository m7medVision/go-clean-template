package eventcodec_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _samplesDir = "testdata/events"

func sampleName(e entity.Event) string {
	return fmt.Sprintf("%s.v%d.json", e.EventType(), e.SchemaVersion())
}

func TestEveryEventTypeStoresTheShapeItsSampleHolds(t *testing.T) {
	t.Parallel()

	codec := eventcodec.New(entity.EventTypes(), entity.Upcasts()...)

	for _, e := range entity.EventTypes() {
		stored, err := codec.Encode(e)
		require.NoError(t, err, e.EventType())

		want, err := os.ReadFile(filepath.Join(_samplesDir, sampleName(e)))
		require.NoError(t, err, "no sample for %s: UPDATE_GOLDEN=1 go test ./internal/repo/eventcodec/ writes it", sampleName(e))
		assert.JSONEq(t, string(want), string(stored.Payload),
			"%s changed its stored shape: raise SchemaVersion(), add an entity.Upcast and keep the old sample", e.EventType())
	}
}

func TestEveryStoredSampleStillDecodes(t *testing.T) {
	t.Parallel()

	codec := eventcodec.New(entity.EventTypes(), entity.Upcasts()...)

	paths, err := filepath.Glob(filepath.Join(_samplesDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		eventType, versionText, found := strings.Cut(name, ".v")
		require.True(t, found, path)

		var version int

		_, err := fmt.Sscanf(versionText, "%d", &version)
		require.NoError(t, err, path)

		payload, err := os.ReadFile(path)
		require.NoError(t, err)

		event, err := codec.Decode(eventcodec.Stored{Type: eventType, SchemaVersion: version, Payload: json.RawMessage(payload)})
		require.NoError(t, err, path)
		assert.Equal(t, eventType, event.EventType())
	}
}

func TestWriteEventSamples(t *testing.T) {
	t.Parallel()

	if os.Getenv("UPDATE_GOLDEN") == "" {
		t.Skip("UPDATE_GOLDEN=1 writes the samples")
	}

	codec := eventcodec.New(entity.EventTypes(), entity.Upcasts()...)

	require.NoError(t, os.MkdirAll(_samplesDir, 0o750))

	for _, e := range entity.EventTypes() {
		stored, err := codec.Encode(e)
		require.NoError(t, err, e.EventType())
		require.NoError(t, os.WriteFile(filepath.Join(_samplesDir, sampleName(e)), stored.Payload, 0o600))
	}
}
