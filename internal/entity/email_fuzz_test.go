package entity_test

import (
	"encoding/json"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

func FuzzDecodeEmail(f *testing.F) {
	for _, e := range entity.EmailTypes() {
		body, err := entity.EncodeEmail(e)
		if err != nil {
			f.Fatal(err)
		}

		f.Add([]byte(body))
	}

	f.Add([]byte(`{"template":"nope","data":{}}`))
	f.Add([]byte(`{"template":"welcome","data":null}`))
	f.Add([]byte(`"not an object"`))

	f.Fuzz(func(t *testing.T, body []byte) {
		email, err := entity.DecodeEmail(json.RawMessage(body))
		if err != nil {
			return
		}

		if email == nil {
			t.Fatal("DecodeEmail returned neither an email nor an error")
		}

		if _, err := entity.EncodeEmail(email); err != nil {
			t.Fatalf("decoded %s but cannot encode it again: %v", email.Template(), err)
		}
	})
}
