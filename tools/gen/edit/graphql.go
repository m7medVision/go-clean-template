package edit

import (
	"fmt"
	"slices"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// GraphQL is a parsed GraphQL schema file, read for what it declares: the generator creates
// schema files and edits none.
type GraphQL struct {
	doc *ast.SchemaDocument
}

// ParseGraphQL parses src, the content of the schema file at path.
func ParseGraphQL(path string, src []byte) (*GraphQL, error) {
	doc, err := parser.ParseSchema(&ast.Source{Name: path, Input: string(src)})
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &GraphQL{doc: doc}, nil
}

// Types returns the names of the types the file defines, not extends, in order.
func (q *GraphQL) Types() []string {
	names := make([]string, 0, len(q.doc.Definitions))
	for _, d := range q.doc.Definitions {
		names = append(names, d.Name)
	}

	return names
}

// Fields returns the fields of every type the file defines or extends, each as Type.field, in
// file order. Only object, interface and input types have fields.
func (q *GraphQL) Fields() []string {
	defs := slices.Concat(q.doc.Definitions, q.doc.Extensions)
	slices.SortStableFunc(defs, func(a, b *ast.Definition) int { return a.Position.Start - b.Position.Start })

	var names []string

	for _, d := range defs {
		for _, f := range d.Fields {
			names = append(names, d.Name+"."+f.Name)
		}
	}

	return names
}
