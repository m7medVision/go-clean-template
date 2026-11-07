package worker_test

import (
	"log/slog"
	"testing"

	workerctl "github.com/m7medvision/go-clean-template/internal/controller/worker"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
)

func TestNewRouterRegistersOnlyTheExampleQueueByDefault(t *testing.T) {
	t.Parallel()

	w := worker.New(slog.New(slog.DiscardHandler), nil)

	workerctl.NewRouter(w, slog.New(slog.DiscardHandler))

	assert.Equal(t, []string{"example"}, w.Queues())
}
