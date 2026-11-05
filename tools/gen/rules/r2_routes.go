package rules

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// _restDir holds every REST operation the app registers.
const _restDir = "internal/controller/restapi"

// operation is one REST operation the app registers, read from its huma.Operation literal.
type operation struct {
	file, id, method, path string
}

// RouteFree checks R2 for the REST operation of a new use case: no operation the app registers has
// its operation ID, or its method and path. Neither clash fails the build: the router keeps the last
// of two operations on one route, and Huma panics on an operation ID registered twice only when the
// router is built, so make openapi or a test would find it, after the write. Two use cases whose
// names start with the same word get the same default path, as count-tasks and count-done-tasks
// both get GET /v1/tasks/count, so the error for a route says to give the use case its own path=.
// Paths that differ only in the names of their parameters are one route. The operations are the
// huma.Operation literals, with a constant Method and Path, in the Go files under
// internal/controller/restapi that are not tests.
func RouteFree(r Repo, method, p, operationID string) error {
	for _, op := range operations(r) {
		switch {
		case op.id == operationID:
			return fmt.Errorf("%w: %s registers the operation ID %s already: give the use case another name",
				ErrTargets, op.file, operationID)
		case op.method == method && sameRoute(op.path, p):
			return fmt.Errorf("%w: %s %s is served already, by the operation %s in %s: give the use case its own path=",
				ErrTargets, method, p, op.id, op.file)
		}
	}

	return nil
}

// operations returns the REST operations the app registers, file by file: those of the restapi
// package, of each version (restapi/v1) and of each domain in a version (restapi/v1/task). A file
// that cannot be read or parsed registers none here; the build reports it.
func operations(r Repo) []operation {
	var ops []operation

	for _, files := range []string{"/*.go", "/*/*.go", "/*/*/*.go"} {
		for _, p := range r.glob(_restDir + files) {
			if !strings.HasSuffix(p, "_test.go") {
				ops = append(ops, fileOperations(r, p)...)
			}
		}
	}

	return ops
}

// fileOperations returns the operations the Go file at p registers: its huma.Operation literals.
func fileOperations(r Repo, p string) []operation {
	src, err := r.Read(p)
	if err != nil {
		return nil
	}

	f, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}

	var ops []operation

	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && isHumaOperation(lit.Type) {
			op := operation{file: p}
			for _, e := range lit.Elts {
				op.set(e)
			}

			ops = append(ops, op)
		}

		return true
	})

	return ops
}

func isHumaOperation(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Operation" {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)

	return ok && pkg.Name == "huma"
}

// set reads one element of a huma.Operation literal into op: its OperationID, Method or Path, when
// it is a constant. A method is a string or a net/http constant, as http.MethodPost is POST.
func (op *operation) set(e ast.Expr) {
	kv, ok := e.(*ast.KeyValueExpr)
	if !ok {
		return
	}

	key, ok := kv.Key.(*ast.Ident)
	if !ok {
		return
	}

	value := constant(kv.Value)

	switch key.Name {
	case "OperationID":
		op.id = value
	case "Method":
		op.method = strings.ToUpper(strings.TrimPrefix(value, "http.Method"))
	case "Path":
		op.path = value
	}
}

// constant is a string literal's value, or a selector such as http.MethodGet as it is written;
// empty for anything else.
func constant(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); err == nil {
			return s
		}
	case *ast.SelectorExpr:
		if pkg, ok := v.X.(*ast.Ident); ok {
			return pkg.Name + "." + v.Sel.Name
		}
	}

	return ""
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
