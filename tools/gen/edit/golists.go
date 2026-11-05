package edit

import (
	"go/ast"
	"go/token"
	"path"
	"strings"
)

// Match picks out the node an insertion goes next to.
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

// Leading matches a composite literal whose first element matches the path.Match pattern, such
// as {entity.ErrReadModelBehind, …}.
func Leading(pattern string) Match {
	return Match{what: "an element led by " + pattern, ok: func(n ast.Node) bool {
		lit, ok := unwrap(n).(*ast.CompositeLit)

		return ok && len(lit.Elts) > 0 && glob(pattern, render(lit.Elts[0]))
	}}
}

// unwrap returns the expression a statement or key-value element carries, or n.
func unwrap(n ast.Node) ast.Node {
	switch x := n.(type) {
	case *ast.ExprStmt:
		return x.X
	case *ast.AssignStmt:
		if len(x.Rhs) == 1 {
			return x.Rhs[0]
		}
	case *ast.KeyValueExpr:
		return x.Value
	}

	return n
}

// render writes an identifier or a selector chain as source, and anything else as "".
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
	}

	return ""
}

func glob(pattern, name string) bool {
	ok, err := path.Match(pattern, name)

	return err == nil && ok
}

// AppendArg adds text as the last argument of the one call to callee, a path.Match pattern, in
// function fn.
func (g *Go) AppendArg(fn, callee, text string) (Insertion, error) {
	body, err := g.body(fn)
	if err != nil {
		return Insertion{}, err
	}

	var found []*ast.CallExpr

	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && glob(callee, render(call.Fun)) {
			found = append(found, call)
		}

		return true
	})

	if len(found) != 1 {
		return Insertion{}, anchorError(g.path, "a call to "+callee+" in "+fn, len(found))
	}

	return g.appendItem(found[0].Args, found[0].Rparen, text), nil
}

// AppendElement adds text as the last element of the one composite literal of type lit in
// function fn.
func (g *Go) AppendElement(fn, lit, text string) (Insertion, error) {
	body, err := g.body(fn)
	if err != nil {
		return Insertion{}, err
	}

	var found []*ast.CompositeLit

	ast.Inspect(body, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok && render(cl.Type) == lit {
			found = append(found, cl)
		}

		return true
	})

	if len(found) != 1 {
		return Insertion{}, anchorError(g.path, "a "+lit+"{…} literal in "+fn, len(found))
	}

	return g.appendItem(found[0].Elts, found[0].Rbrace, text), nil
}

// appendItem adds text after the last of items, inside the list that closes at closing: on the
// same line in a one-line list, on a line of its own with a trailing comma in a multi-line one.
func (g *Go) appendItem(items []ast.Expr, closing token.Pos, text string) Insertion {
	end := g.offset(closing)

	switch {
	case len(items) == 0:
		return g.at(end, text)
	case g.line(items[len(items)-1].End()) == g.line(closing):
		return g.at(g.offset(items[len(items)-1].End()), ", "+text)
	default:
		return g.at(lineStart(g.src, end), indent(text+",", lineIndent(g.src, end)+"\t"))
	}
}

// AfterLastElement inserts text, with a trailing comma, on the line after the last element that
// matches m. The matches must all be in one composite literal in function fn.
func (g *Go) AfterLastElement(fn string, m Match, text string) (Insertion, error) {
	elts, err := g.matchingElements(fn, m)
	if err != nil {
		return Insertion{}, err
	}

	last := elts[len(elts)-1]

	return g.at(nextLineStart(g.src, g.offset(last.End())), indent(text+",", lineIndent(g.src, g.offset(last.Pos())))), nil
}

// BeforeFirstElement inserts text, with a trailing comma, on the line before the first element
// that matches m, and before the comment just above it. The matches must all be in one
// composite literal in function fn.
func (g *Go) BeforeFirstElement(fn string, m Match, text string) (Insertion, error) {
	elts, err := g.matchingElements(fn, m)
	if err != nil {
		return Insertion{}, err
	}

	first := elts[0]
	at := lineStart(g.src, g.offset(first.Pos()))

	for _, c := range g.file.Comments {
		start := lineStart(g.src, g.offset(c.Pos()))
		ownLine := strings.TrimSpace(string(g.src[start:g.offset(c.Pos())])) == ""

		if ownLine && g.line(c.End()) == g.line(first.Pos())-1 {
			at = start
		}
	}

	return g.at(at, indent(text+",", lineIndent(g.src, g.offset(first.Pos())))), nil
}

func (g *Go) matchingElements(fn string, m Match) ([]ast.Expr, error) {
	body, err := g.body(fn)
	if err != nil {
		return nil, err
	}

	var found [][]ast.Expr

	ast.Inspect(body, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		var elts []ast.Expr

		for _, e := range cl.Elts {
			if m.ok(e) {
				elts = append(elts, e)
			}
		}

		if len(elts) > 0 {
			found = append(found, elts)
		}

		return true
	})

	if len(found) != 1 {
		return nil, anchorError(g.path, "a literal in "+fn+" holding "+m.what, len(found))
	}

	return found[0], nil
}

// AfterLastStmt inserts text on the line after the last statement of function fn's body that
// matches m, at that statement's indentation. Only the body's own statements count, not those
// nested in blocks.
func (g *Go) AfterLastStmt(fn string, m Match, text string) (Insertion, error) {
	body, err := g.body(fn)
	if err != nil {
		return Insertion{}, err
	}

	var last ast.Stmt

	for _, s := range body.List {
		if m.ok(s) {
			last = s
		}
	}

	if last == nil {
		return Insertion{}, anchorError(g.path, "a statement in "+fn+" that is "+m.what, 0)
	}

	return g.at(nextLineStart(g.src, g.offset(last.End())), indent(text, lineIndent(g.src, g.offset(last.Pos())))), nil
}

// AppendField adds text as the last field of the multi-line struct type named name.
func (g *Go) AppendField(name, text string) (Insertion, error) {
	var found []*ast.StructType

	for _, ts := range g.typeSpecs() {
		st, ok := ts.Type.(*ast.StructType)
		if ok && ts.Name.Name == name && g.line(st.Fields.Opening) != g.line(st.Fields.Closing) {
			found = append(found, st)
		}
	}

	if len(found) != 1 {
		return Insertion{}, anchorError(g.path, "a multi-line struct type "+name, len(found))
	}

	closing := g.offset(found[0].Fields.Closing)

	return g.at(lineStart(g.src, closing), indent(text, lineIndent(g.src, closing)+"\t")), nil
}

// Field is one field of a struct type: its name and its type as source, as in Tasks and
// repo.TaskReadModel.
type Field struct {
	Name, Type string
}

// FieldOfType returns the one field of the struct type named name whose type, written as pkg.Type
// or Type, matches the path.Match pattern, as repo.*ReadModel matches repo.TaskReadModel. An
// embedded field is named after its type.
func (g *Go) FieldOfType(name, pattern string) (Field, error) {
	var structs []*ast.StructType

	for _, ts := range g.typeSpecs() {
		if st, ok := ts.Type.(*ast.StructType); ok && ts.Name.Name == name {
			structs = append(structs, st)
		}
	}

	if len(structs) != 1 {
		return Field{}, anchorError(g.path, "a struct type "+name, len(structs))
	}

	var found []Field

	for _, f := range structs[0].Fields.List {
		typ := render(f.Type)
		if !glob(pattern, typ) {
			continue
		}

		if len(f.Names) == 0 {
			found = append(found, Field{Name: typ[strings.LastIndexByte(typ, '.')+1:], Type: typ})
		}

		for _, id := range f.Names {
			found = append(found, Field{Name: id.Name, Type: typ})
		}
	}

	if len(found) != 1 {
		return Field{}, anchorError(g.path, "a field of "+name+" of type "+pattern, len(found))
	}

	return found[0], nil
}

// typeSpecs returns the file's top-level type declarations.
func (g *Go) typeSpecs() []*ast.TypeSpec {
	var specs []*ast.TypeSpec

	for _, d := range g.file.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, s := range gd.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok {
					specs = append(specs, ts)
				}
			}
		}
	}

	return specs
}

// AtEnd appends text to the file, after a blank line.
func (g *Go) AtEnd(text string) Insertion {
	return g.at(len(g.src), "\n"+indent(text, ""))
}

func (g *Go) body(fn string) (*ast.BlockStmt, error) {
	fd, err := g.funcDecl(fn)
	if err != nil {
		return nil, err
	}

	if fd.Body == nil {
		return nil, anchorError(g.path, "the body of func "+fn, 0)
	}

	return fd.Body, nil
}

func (g *Go) line(p token.Pos) int {
	return g.fset.Position(p).Line
}
