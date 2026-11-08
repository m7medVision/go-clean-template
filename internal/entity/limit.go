package entity

import "time"

var ErrTooManyAttempts = newError(KindRateLimited, "too many attempts")

type RetryAfterError struct {
	RetryAfter time.Duration
}

func TooManyAttempts(retryAfter time.Duration) error {
	return &RetryAfterError{RetryAfter: retryAfter}
}

func (e *RetryAfterError) Error() string { return ErrTooManyAttempts.Error() }

func (e *RetryAfterError) Unwrap() error { return ErrTooManyAttempts }

type Allowance struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limits struct {
	ClientAttempts  int
	AccountFailures int
	Window          time.Duration
}
