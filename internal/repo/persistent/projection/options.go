package projection

import "time"

// Option configures a Runner.
type Option func(*Runner)

// PollInterval is how long an idle subscriber waits for a notification before it reads again.
func PollInterval(d time.Duration) Option {
	return func(r *Runner) {
		r.pollInterval = d
	}
}

// BatchSize is the most events a subscriber reads at once.
func BatchSize(n int) Option {
	return func(r *Runner) {
		r.batchSize = n
	}
}
