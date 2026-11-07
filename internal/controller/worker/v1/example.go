package v1

import (
	"context"
	"encoding/json"

	"github.com/m7medvision/go-clean-template/internal/controller/worker/v1/request"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// Handle logs the message. A real handler decodes the body the same way, calls a use case with ctx,
// and returns its error so the message is redelivered.
func (c *ExampleController) Handle(_ context.Context, msg *worker.Message) error {
	var body request.Example

	// A message that does not decode will never decode, so it is dropped rather than retried.
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		c.l.Error("worker - v1 - example - decode", "message", msg.ID, "error", err)

		return nil
	}

	c.l.Info("worker - v1 - example", "message", msg.ID, "reads", msg.ReadCount, "text", body.Text)

	return nil
}
