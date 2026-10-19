package worker

import "time"

// Option -.
type Option func(*Worker)

// PollInterval is how long a queue loop sleeps after a read returns no messages.
func PollInterval(d time.Duration) Option {
	return func(w *Worker) {
		w.pollInterval = d
	}
}

// VisibilityTimeout is how long a read message stays hidden from other readers. A message whose
// handler fails is redelivered once it expires.
func VisibilityTimeout(d time.Duration) Option {
	return func(w *Worker) {
		w.visibilityTimeout = d
	}
}

// BatchSize is the maximum number of messages read per poll.
func BatchSize(n int) Option {
	return func(w *Worker) {
		w.batchSize = n
	}
}

// ShutdownTimeout is how long Shutdown waits for in-flight handlers before canceling their context.
func ShutdownTimeout(d time.Duration) Option {
	return func(w *Worker) {
		w.shutdownTimeout = d
	}
}
