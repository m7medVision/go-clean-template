package edit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// Go is a parsed Go file that changes are planned against.
type Go struct {
	path string
	src  []byte
	fset *token.FileSet
	file *ast.File
}

// ParseGo parses src, the content of the Go file at path.
func ParseGo(path string, src []byte) (*Go, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &Go{path: path, src: src, fset: fset, file: file}, nil
}

func (g *Go) offset(p token.Pos) int {
	return g.fset.Position(p).Offset
}

func (g *Go) line(p token.Pos) int {
	return g.fset.Position(p).Line
}

func (g *Go) at(offset int, text string) Insertion {
	return Insertion{Offset: offset, Text: text}
}

// BeforeBlockEnd inserts text as the last lines of the file's one parenthesized block of tok, such
// as a type ( … ), indented one tab.
func (g *Go) BeforeBlockEnd(tok token.Token, text string) (Insertion, error) {
	var found []*ast.GenDecl

	for _, d := range g.file.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == tok && gd.Rparen.IsValid() {
			found = append(found, gd)
		}
	}

	if len(found) != 1 {
		return Insertion{}, anchorError(g.path, fmt.Sprintf("a %s ( … ) block", tok), len(found))
	}

	return g.at(lineStart(g.src, g.offset(found[0].Rparen)), indent(text, "\t")), nil
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

// AppendElement adds text as the last element of the one composite literal of type lit, such as
// []module, in function fn.
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

// AfterLastStmt inserts text on the line after the last statement of function fn's body that
// matches m, at that statement's indentation. Only the body's own statements count, not those
// nested in blocks. The place is the end of the statements that match, which must be one run, one
// statement after another, as a router's registrations are: two runs, split by a statement that
// does not match, would be two places to choose from.
func (g *Go) AfterLastStmt(fn string, m Match, text string) (Insertion, error) {
	body, err := g.body(fn)
	if err != nil {
		return Insertion{}, err
	}

	var (
		last ast.Stmt
		runs int
	)

	for i, s := range body.List {
		if !m.ok(s) {
			continue
		}

		if i == 0 || !m.ok(body.List[i-1]) {
			runs++
		}

		last = s
	}

	if runs != 1 {
		return Insertion{}, anchorError(g.path, "a run of statements in "+fn+" that are each "+m.what, runs)
	}

	return g.at(nextLineStart(g.src, g.offset(last.End())), indent(text, lineIndent(g.src, g.offset(last.Pos())))), nil
}

// AppendField adds text as the last field of the multi-line struct type named name.
func (g *Go) AppendField(name, text string) (Insertion, error) {
	var found []*ast.StructType

	for _, d := range g.file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}

		for _, s := range gd.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}

			if st, ok := ts.Type.(*ast.StructType); ok && g.line(st.Fields.Opening) != g.line(st.Fields.Closing) {
				found = append(found, st)
			}
		}
	}

	if len(found) != 1 {
		return Insertion{}, anchorError(g.path, "a multi-line struct type "+name, len(found))
	}

	closing := g.offset(found[0].Fields.Closing)

	return g.at(lineStart(g.src, closing), indent(text, lineIndent(g.src, closing)+"\t")), nil
}

// body is the body of the file's one function or method called fn.
func (g *Go) body(fn string) (*ast.BlockStmt, error) {
	var found []*ast.FuncDecl

	for _, d := range g.file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == fn && fd.Body != nil {
			found = append(found, fd)
		}
	}

	if len(found) != 1 {
		return nil, anchorError(g.path, "func "+fn, len(found))
	}

	return found[0].Body, nil
}

// Import returns the insertion that imports path, or none when the file already does. It goes into its group, the standard library's or the other imports', in
// path order, the way gci sorts them. A file with no standard import starts that group ahead of
// the others, and one with no other import starts theirs after it.
func (g *Go) Import(path string) ([]Insertion, error) {
	block, err := g.importBlock()
	if err != nil {
		return nil, err
	}

	if present, err := g.imported(block, path); err != nil || present {
		return nil, err
	}

	spec := strconv.Quote(path)

	before, last := importNeighbours(block, path)

	switch {
	case before != nil:
		return []Insertion{g.at(lineStart(g.src, g.offset(before.Pos())), indent(spec, "\t"))}, nil
	case last != nil:
		return []Insertion{g.at(nextLineStart(g.src, g.offset(last.End())), indent(spec, "\t"))}, nil
	case !isStandard(path):
		return []Insertion{g.at(lineStart(g.src, g.offset(block.Rparen)), "\n"+indent(spec, "\t"))}, nil
	case len(block.Specs) > 0:
		return []Insertion{g.at(lineStart(g.src, g.offset(block.Specs[0].Pos())), indent(spec, "\t")+"\n")}, nil
	default:
		return []Insertion{g.at(lineStart(g.src, g.offset(block.Rparen)), indent(spec, "\t"))}, nil
	}
}

func (g *Go) importBlock() (*ast.GenDecl, error) {
	var found []*ast.GenDecl

	for _, d := range g.file.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT && gd.Rparen.IsValid() {
			found = append(found, gd)
		}
	}

	if len(found) != 1 {
		return nil, anchorError(g.path, "an import ( … ) block", len(found))
	}

	return found[0], nil
}

// imported reports whether block imports path under alias. Importing it under another alias is
// an error: the generated code would not compile against it. So is another import that binds the
// name path would be imported as.
func (g *Go) imported(block *ast.GenDecl, path string) (bool, error) {
	name := importName(path, "")

	for _, s := range block.Specs {
		is, ok := s.(*ast.ImportSpec)
		if !ok {
			continue
		}

		if importPath(is) != path {
			if bound := importName(importPath(is), aliasOf(is)); bound == name {
				return false, fmt.Errorf("%w: %s: %s is imported already, from %s, so %s cannot be",
					ErrAnchor, g.path, name, importPath(is), path)
			}

			continue
		}

		if aliasOf(is) == "" {
			return true, nil
		}

		return false, fmt.Errorf("%w: %s: imports %s under the alias %s", ErrAnchor, g.path, path, aliasOf(is))
	}

	return false, nil
}

func aliasOf(is *ast.ImportSpec) string {
	if is.Name == nil {
		return ""
	}

	return is.Name.Name
}

// importName is the name an import of path under alias binds in the file: the alias, or the last
// element of the path that is not a major version such as v2, which the package is named after in
// this repository.
func importName(path, alias string) string {
	if alias != "" {
		return alias
	}

	elems := strings.Split(path, "/")
	last := elems[len(elems)-1]

	if len(elems) > 1 && isMajorVersion(last) {
		last = elems[len(elems)-2]
	}

	base, _, _ := strings.Cut(last, ".")

	return base
}

// isMajorVersion reports whether a path element is a major version suffix, as v2 is.
func isMajorVersion(elem string) bool {
	if len(elem) < len("v2") || elem[0] != 'v' {
		return false
	}

	_, err := strconv.Atoi(elem[1:])

	return err == nil
}

// importNeighbours returns, among the imports in path's group (the standard library's or the
// others'), the first that sorts after path and the last.
func importNeighbours(block *ast.GenDecl, path string) (before, last *ast.ImportSpec) {
	for _, s := range block.Specs {
		is, ok := s.(*ast.ImportSpec)
		if !ok || isStandard(importPath(is)) != isStandard(path) {
			continue
		}

		last = is
		if before == nil && importPath(is) > path {
			before = is
		}
	}

	return before, last
}

func importPath(is *ast.ImportSpec) string {
	p, err := strconv.Unquote(is.Path.Value)
	if err != nil {
		return is.Path.Value
	}

	return p
}

// isStandard reports whether path is in the standard library: its first element has no dot.
func isStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")

	return !strings.Contains(first, ".")
}
