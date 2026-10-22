package worker_test

import (
	"testing"

	workerctl "github.com/m7medvision/go-clean-template/internal/controller/worker"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
)

func TestWithMailRegistersTheMailQueue(t *testing.T) {
	t.Parallel()

	w := worker.New(logger.New("error"), nil)

	workerctl.NewRouter(w, logger.New("error"), workerctl.WithMail(nil))

	assert.Equal(t, []string{"example", "mail"}, w.Queues())
}
