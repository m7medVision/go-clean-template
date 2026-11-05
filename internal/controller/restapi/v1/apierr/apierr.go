// Package apierr maps the failures any command can produce, whatever its domain, to REST problems.
// Each operation still maps its own domain's errors inline.
package apierr

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// Shared returns the problem for a failure every command can produce, and nil for any other:
//   - entity.ErrReadModelBehind is 503: the write is saved but a read could still miss it, so the
//     caller should read again shortly rather than write again;
//   - entity.ErrConcurrencyConflict is 409: the record changed meanwhile, even after retries;
//   - entity.ErrNotImplemented is 501: the handler is still a generated stub.
func Shared(err error) error {
	switch {
	case errors.Is(err, entity.ErrReadModelBehind):
		return huma.Error503ServiceUnavailable("saved, but not yet readable: read it again shortly")
	case errors.Is(err, entity.ErrConcurrencyConflict):
		return huma.Error409Conflict("the record changed while this request ran: try again")
	case errors.Is(err, entity.ErrNotImplemented):
		return huma.Error501NotImplemented("not implemented")
	default:
		return nil
	}
}
