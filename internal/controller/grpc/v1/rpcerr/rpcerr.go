// Package rpcerr maps the failures any command can produce, whatever its domain, to gRPC statuses.
// Each RPC still maps its own domain's errors inline.
package rpcerr

import (
	"errors"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Shared returns the status for a failure every command can produce, and nil for any other:
// entity.ErrReadModelBehind is Unavailable (saved, read again shortly), and
// entity.ErrConcurrencyConflict is Aborted (the record changed meanwhile, even after retries).
func Shared(err error) error {
	switch {
	case errors.Is(err, entity.ErrReadModelBehind):
		return status.Error(codes.Unavailable, "saved, but not yet readable: read it again shortly")
	case errors.Is(err, entity.ErrConcurrencyConflict):
		return status.Error(codes.Aborted, "the record changed while this request ran: try again")
	default:
		return nil
	}
}
