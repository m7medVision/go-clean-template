package eventcodec_test

import (
	"encoding/json"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
)

func FuzzCodecDecode(f *testing.F) {
	events := entity.EventTypes()
	codec := eventcodec.New(events, entity.Upcasts()...)

	for _, e := range events {
		stored, err := codec.Encode(e)
		if err != nil {
			f.Fatal(err)
		}

		f.Add(stored.Type, stored.SchemaVersion, []byte(stored.Payload))
	}

	f.Add("Unknown", 1, []byte(`{}`))
	f.Add("", -1, []byte(`null`))

	f.Fuzz(func(t *testing.T, eventType string, version int, payload []byte) {
		event, err := codec.Decode(eventcodec.Stored{Type: eventType, SchemaVersion: version, Payload: json.RawMessage(payload)})
		if err != nil {
			return
		}

		if _, err := codec.Encode(event); err != nil {
			t.Fatalf("decoded %s but cannot encode it again: %v", eventType, err)
		}
	})
}
