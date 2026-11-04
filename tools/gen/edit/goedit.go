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

func (g *Go) at(offset int, text string) Insertion {
	return Insertion{Path: g.path, Offset: offset, Text: text}
}

// BeforeBlockEnd inserts text as the last lines of the file's one parenthesized block of tok, a
// var ( … ) or a type ( … ), indented one tab.
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

// Import returns the insertion that imports path, under alias when it is set, or none when the
// file already does. It goes into the group of non-standard imports in path order, the way gci
// sorts them, starting that group when the file has none.
func (g *Go) Import(path, alias string) ([]Insertion, error) {
	block, err := g.importBlock()
	if err != nil {
		return nil, err
	}

	if present, err := g.imported(block, path, alias); err != nil || present {
		return nil, err
	}

	spec := strconv.Quote(path)
	if alias != "" {
		spec = alias + " " + spec
	}

	before, last := importNeighbours(block, path)

	switch {
	case before != nil:
		return []Insertion{g.at(lineStart(g.src, g.offset(before.Pos())), indent(spec, "\t"))}, nil
	case last != nil:
		return []Insertion{g.at(nextLineStart(g.src, g.offset(last.End())), indent(spec, "\t"))}, nil
	default:
		return []Insertion{g.at(lineStart(g.src, g.offset(block.Rparen)), "\n"+indent(spec, "\t"))}, nil
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
func (g *Go) imported(block *ast.GenDecl, path, alias string) (bool, error) {
	name := importName(path, alias)

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

		if aliasOf(is) == alias {
			return true, nil
		}

		return false, fmt.Errorf("%w: %s: imports %s under another alias than %q", ErrAnchor, g.path, path, alias)
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

// importNeighbours returns the first non-standard import that sorts after path, and the last
// non-standard import.
func importNeighbours(block *ast.GenDecl, path string) (before, last *ast.ImportSpec) {
	for _, s := range block.Specs {
		is, ok := s.(*ast.ImportSpec)
		if !ok || isStandard(importPath(is)) {
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

// RequireFunc checks that the file declares exactly one function or method called name.
func (g *Go) RequireFunc(name string) error {
	_, err := g.funcDecl(name)

	return err
}

func (g *Go) funcDecl(name string) (*ast.FuncDecl, error) {
	var found []*ast.FuncDecl

	for _, d := range g.file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			found = append(found, fd)
		}
	}

	if len(found) != 1 {
		return nil, anchorError(g.path, "func "+name, len(found))
	}

	return found[0], nil
}
