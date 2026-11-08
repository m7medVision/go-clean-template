// Package apierr turns a failed command or query into its REST problem, from the kind of its
// entity.Error, for every domain.
package apierr

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// Problem is the RFC 9457 problem for err, logged under op as its kind says.
func Problem(ctx context.Context, l *slog.Logger, err error, op string) error {
	kind := entity.KindOf(err)
	if level, ok := kind.LogLevel(); ok {
		l.Log(ctx, level, "restapi - "+op, "error", err)
	}

	problem := huma.NewError(status(kind), entity.Message(err))
	if retry, ok := errors.AsType[*entity.RetryAfterError](err); ok {
		return huma.ErrorWithHeaders(problem, http.Header{"Retry-After": {retryAfterSeconds(retry.RetryAfter)}})
	}

	return problem
}

func retryAfterSeconds(d time.Duration) string {
	return strconv.Itoa(max(1, int(math.Ceil(d.Seconds()))))
}

// status is the HTTP status of a kind. A switch, so that exhaustive fails the lint on a kind it
// does not name.
func status(k entity.Kind) int { //nolint:gocyclo,cyclop // exhaustive needs every kind named, and each status is a case of its own
	switch k {
	case entity.KindInvalid:
		return http.StatusBadRequest
	case entity.KindUnauthenticated:
		return http.StatusUnauthorized
	case entity.KindForbidden:
		return http.StatusForbidden
	case entity.KindNotFound:
		return http.StatusNotFound
	case entity.KindExists, entity.KindConflict:
		return http.StatusConflict
	case entity.KindUnavailable:
		return http.StatusServiceUnavailable
	case entity.KindUpstream:
		return http.StatusBadGateway
	case entity.KindNotImplemented:
		return http.StatusNotImplemented
	case entity.KindRateLimited:
		return http.StatusTooManyRequests
	case entity.KindInternal:
	}

	return http.StatusInternalServerError
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
