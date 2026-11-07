// Package rpcerr turns a failed command or query into its gRPC status, from the kind of its
// entity.Error, for every service.
package rpcerr

import (
	"context"
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Status is the gRPC status for err, logged under op as its kind says.
func Status(ctx context.Context, l *slog.Logger, err error, op string) error {
	kind := entity.KindOf(err)
	if level, ok := kind.LogLevel(); ok {
		l.Log(ctx, level, "grpc - "+op, "error", err)
	}

	return status.Error(code(kind), entity.Message(err))
}

// code is the gRPC code of a kind. A switch, so that exhaustive fails the lint on a kind it does
// not name.
func code(k entity.Kind) codes.Code {
	switch k {
	case entity.KindInvalid:
		return codes.InvalidArgument
	case entity.KindUnauthenticated:
		return codes.Unauthenticated
	case entity.KindForbidden:
		return codes.PermissionDenied
	case entity.KindNotFound:
		return codes.NotFound
	case entity.KindExists:
		return codes.AlreadyExists
	case entity.KindConflict:
		return codes.Aborted
	case entity.KindUnavailable, entity.KindUpstream:
		return codes.Unavailable
	case entity.KindNotImplemented:
		return codes.Unimplemented
	case entity.KindInternal:
	}

	return codes.Internal
}
