package rules

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// _stale is what to do when the OpenAPI document no longer says what the app registers.
const _stale = "if the document is stale, run make openapi first"

// operation is one REST operation the app registers, as the OpenAPI document lists it.
type operation struct {
	id, method, path string
}

// RouteFree checks R2 for the REST operation of a new use case: no operation the app registers has
// its operation ID, or its method and path. Neither clash fails the build: the router keeps the last
// of two operations on one route, and Huma panics on an operation ID registered twice only when the
// router is built, so make openapi or a test would find it, after the write. Two use cases whose
// names start with the same word get the same default path, as count-tasks and count-done-tasks
// both get GET /v1/tasks/count, so the error for a route says to give the use case its own path=.
// Paths that differ only in the names of their parameters are one route. The operations are those
// docs/openapi/v1.json lists, which make openapi exports from the REST API as the server builds it,
// so the document must be current: CI fails when it is not, and make new-domain and make
// new-usecase export it again after they write.
func RouteFree(r Repo, method, p, operationID string) error {
	ops, err := operations(r)
	if err != nil {
		return err
	}

	for _, op := range ops {
		switch {
		case op.id == operationID:
			return fmt.Errorf("%w: %s lists the operation ID %s already, for %s %s: give the use case another name (%s)",
				ErrTargets, _openAPI, operationID, op.method, op.path, _stale)
		case op.method == method && sameRoute(op.path, p):
			return fmt.Errorf("%w: %s %s is served already, by the operation %s in %s: give the use case its own path= (%s)",
				ErrTargets, method, p, op.id, _openAPI, _stale)
		}
	}

	return nil
}

// operations returns the REST operations the OpenAPI document lists, by path and then method: the
// HTTP methods of each path item, read with their operationId.
func operations(r Repo) ([]operation, error) {
	src, err := fs.ReadFile(r.FS, _openAPI)
	if err != nil {
		return nil, fmt.Errorf("read %s, which lists the REST operations the app registers: %w: run make openapi to export it",
			_openAPI, err)
	}

	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}

	if err := json.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w: run make openapi to export it again", _openAPI, err)
	}

	var ops []operation

	for p, item := range doc.Paths {
		for method, raw := range item {
			if !slices.Contains([]string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}, method) {
				continue // another field of the path item, such as its parameters
			}

			var op struct {
				ID string `json:"operationId"`
			}

			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("parse %s: %s %s: %w: run make openapi to export it again", _openAPI, method, p, err)
			}

			ops = append(ops, operation{id: op.ID, method: strings.ToUpper(method), path: p})
		}
	}

	slices.SortFunc(ops, func(a, b operation) int {
		return cmp.Or(strings.Compare(a.path, b.path), strings.Compare(a.method, b.method))
	})

	return ops, nil
}

// sameRoute reports whether the paths a and b are one route: alike but for the names of their
// parameters, as /v1/tasks/{id} and /v1/tasks/{taskId} are.
func sameRoute(a, b string) bool {
	return a != "" && unnamed(a) == unnamed(b)
}

// unnamed is the path p with each parameter's name left out, as /v1/tasks/{} for /v1/tasks/{id}.
func unnamed(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			segments[i] = "{}"
		}
	}

	return strings.Join(segments, "/")
}
