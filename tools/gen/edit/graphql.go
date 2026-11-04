package edit

import (
	"fmt"
	"unicode/utf8"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/lexer"
	"github.com/vektah/gqlparser/v2/parser"
)

// GraphQL is a parsed GraphQL schema file that changes are planned against.
type GraphQL struct {
	path   string
	src    []byte
	doc    *ast.SchemaDocument
	tokens []lexer.Token
	// byteAt maps a rune offset, which the lexer reports, to a byte offset into src.
	byteAt []int
}

// ParseGraphQL parses src, the content of the schema file at path.
func ParseGraphQL(path string, src []byte) (*GraphQL, error) {
	source := &ast.Source{Name: path, Input: string(src)}

	doc, err := parser.ParseSchema(source)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	lex := lexer.New(source)

	var tokens []lexer.Token

	for {
		tok, err := lex.ReadToken()
		if err != nil {
			return nil, fmt.Errorf("lex %s: %w", path, err)
		}

		if tok.Kind == lexer.EOF {
			break
		}

		if tok.Kind != lexer.Comment {
			tokens = append(tokens, tok)
		}
	}

	byteAt := make([]int, 0, utf8.RuneCount(src)+1)
	for i := range string(src) {
		byteAt = append(byteAt, i)
	}

	byteAt = append(byteAt, len(src))

	return &GraphQL{path: path, src: src, doc: doc, tokens: tokens, byteAt: byteAt}, nil
}

// InType inserts text as the last lines of the file's one definition of the type called name,
// written type or extend type, indented two spaces. The closing brace is found by matching
// braces in the token stream, where strings and comments are single tokens.
func (q *GraphQL) InType(name, text string) (Insertion, error) {
	var closings []int

	for i := 0; i+1 < len(q.tokens); i++ {
		keyword, typeName := q.tokens[i], q.tokens[i+1]
		if keyword.Value == "type" && typeName.Value == name && q.atTopLevel(i) {
			if closing, ok := q.closingBrace(i + 1); ok {
				closings = append(closings, closing)
			}
		}
	}

	if len(closings) != 1 {
		return Insertion{}, anchorError(q.path, "type "+name+" { … }", len(closings))
	}

	at := q.byteAt[q.tokens[closings[0]].Pos.Start]

	return Insertion{Path: q.path, Offset: lineStart(q.src, at), Text: indent(text, "  ")}, nil
}

// atTopLevel reports whether token i is outside every brace.
func (q *GraphQL) atTopLevel(i int) bool {
	depth := 0

	for _, t := range q.tokens[:i] {
		switch t.Kind { //nolint:exhaustive // only braces change the depth
		case lexer.BraceL:
			depth++
		case lexer.BraceR:
			depth--
		}
	}

	return depth == 0
}

// closingBrace returns the index of the brace that closes the block opened by the first brace
// at or after token from. A definition with no block before the next definition has none.
func (q *GraphQL) closingBrace(from int) (int, bool) {
	depth := 0

	for i := from; i < len(q.tokens); i++ {
		switch q.tokens[i].Kind { //nolint:exhaustive // only braces and the next definition matter
		case lexer.BraceL:
			depth++
		case lexer.BraceR:
			depth--
			if depth == 0 {
				return i, true
			}
		case lexer.Name:
			if depth == 0 && isDefinitionKeyword(q.tokens[i].Value) {
				return 0, false
			}
		}
	}

	return 0, false
}

func isDefinitionKeyword(s string) bool {
	switch s {
	case "type", "extend", "input", "enum", "interface", "union", "scalar", "schema", "directive":
		return true
	default:
		return false
	}
}

// AtEnd appends text to the file, after a blank line.
func (q *GraphQL) AtEnd(text string) Insertion {
	return Insertion{Path: q.path, Offset: len(q.src), Text: "\n" + indent(text, "")}
}

// Types returns the names of the types the file defines, not extends, in order.
func (q *GraphQL) Types() []string {
	names := make([]string, 0, len(q.doc.Definitions))
	for _, d := range q.doc.Definitions {
		names = append(names, d.Name)
	}

	return names
}
