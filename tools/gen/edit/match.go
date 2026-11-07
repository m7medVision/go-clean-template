package edit

import (
	"go/ast"
	"path"
)

// Match picks out the statement an insertion goes after.
type Match struct {
	what string
	ok   func(ast.Node) bool
}

// Call matches a call whose callee, written pkg.Func or recv.Method, matches the path.Match
// pattern; a statement that is the call or assigns its result matches too.
func Call(pattern string) Match {
	return Match{what: "a call to " + pattern, ok: func(n ast.Node) bool {
		call, ok := unwrap(n).(*ast.CallExpr)

		return ok && glob(pattern, render(call.Fun))
	}}
}

// unwrap returns the expression a statement carries, or n.
func unwrap(n ast.Node) ast.Node {
	switch x := n.(type) {
	case *ast.ExprStmt:
		return x.X
	case *ast.AssignStmt:
		if len(x.Rhs) == 1 {
			return x.Rhs[0]
		}
	}

	return n
}

// render writes an identifier, a selector chain or a slice type as source, and anything else as "".
func render(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if base := render(x.X); base != "" {
			return base + "." + x.Sel.Name
		}
	case *ast.IndexExpr:
		return render(x.X)
	case *ast.IndexListExpr:
		return render(x.X)
	case *ast.ArrayType:
		if elem := render(x.Elt); x.Len == nil && elem != "" {
			return "[]" + elem
		}
	}

	return ""
}

func glob(pattern, name string) bool {
	ok, err := path.Match(pattern, name)

	return err == nil && ok
}
