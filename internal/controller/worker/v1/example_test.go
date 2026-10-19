package v1_test

import (
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/evrone/go-clean-template/internal/controller/worker/v1"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/require"
)

func TestExampleController_Handle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "well-formed message is acknowledged", body: `{"text":"hello"}`},
		{name: "malformed message is dropped, retrying cannot fix it", body: `"not an object"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := v1.NewExampleController(logger.New("error"))

			err := c.Handle(context.Background(), &worker.Message{ID: 1, Body: json.RawMessage(tc.body)})

			require.NoError(t, err)
		})
	}
}
