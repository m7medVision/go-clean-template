package sso

import "time"

// SetTimeout bounds each step's calls to the provider by d instead of Timeout, so a test of a provider
// that does not answer does not wait Timeout for it.
func (s *SSO) SetTimeout(d time.Duration) {
	s.timeout = d
}
