// Package apierr turns a failed command or query into its REST problem, from the kind of its
// entity.Error, for every domain.
package apierr

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// Problem is the RFC 9457 problem for err, logged under op as its kind says.
func Problem(ctx context.Context, l *slog.Logger, err error, op string) error {
	kind := entity.KindOf(err)
	if level, ok := kind.LogLevel(); ok {
		l.Log(ctx, level, "restapi - "+op, "error", err)
	}

	return huma.NewError(status(kind), entity.Message(err))
}

func status(k entity.Kind) int {
	return [...]int{
		entity.KindInternal:        http.StatusInternalServerError,
		entity.KindInvalid:         http.StatusBadRequest,
		entity.KindUnauthenticated: http.StatusUnauthorized,
		entity.KindForbidden:       http.StatusForbidden,
		entity.KindNotFound:        http.StatusNotFound,
		entity.KindExists:          http.StatusConflict,
		entity.KindConflict:        http.StatusConflict,
		entity.KindUnavailable:     http.StatusServiceUnavailable,
		entity.KindUpstream:        http.StatusBadGateway,
		entity.KindNotImplemented:  http.StatusNotImplemented,
	}[k]
}

// CommandErrors lists the statuses of an operation that sends a command: its own, then the two any
// command can answer, a conflict that outlived the retries (409) and a read model still behind the
// write (503), then 500.
func CommandErrors(statuses ...int) []int {
	return errorList(slices.Concat(statuses, []int{http.StatusConflict, http.StatusServiceUnavailable}))
}

// QueryErrors lists the statuses of an operation that only sends queries: its own, then 500.
func QueryErrors(statuses ...int) []int {
	return errorList(statuses)
}

func errorList(statuses []int) []int {
	statuses = append(statuses, http.StatusInternalServerError)
	slices.Sort(statuses)

	return slices.Compact(statuses)
}
