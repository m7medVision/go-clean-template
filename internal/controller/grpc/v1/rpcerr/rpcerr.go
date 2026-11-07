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

func code(k entity.Kind) codes.Code {
	return [...]codes.Code{
		entity.KindInternal:        codes.Internal,
		entity.KindInvalid:         codes.InvalidArgument,
		entity.KindUnauthenticated: codes.Unauthenticated,
		entity.KindForbidden:       codes.PermissionDenied,
		entity.KindNotFound:        codes.NotFound,
		entity.KindExists:          codes.AlreadyExists,
		entity.KindConflict:        codes.Aborted,
		entity.KindUnavailable:     codes.Unavailable,
		entity.KindUpstream:        codes.Unavailable,
		entity.KindNotImplemented:  codes.Unimplemented,
	}[k]
}
