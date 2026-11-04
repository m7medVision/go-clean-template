package edit

import (
	"bytes"
	"fmt"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
)

// Proto is a parsed .proto file that changes are planned against.
type Proto struct {
	path string
	src  []byte
	file *ast.FileNode
}

// ParseProto parses src, the content of the .proto file at path.
func ParseProto(path string, src []byte) (*Proto, error) {
	file, err := parser.Parse(path, bytes.NewReader(src), reporter.NewHandler(nil))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &Proto{path: path, src: src, file: file}, nil
}

// InService inserts text as the last lines of the one service called name, indented two spaces.
func (p *Proto) InService(name, text string) (Insertion, error) {
	var found []*ast.ServiceNode

	for _, d := range p.file.Decls {
		if s, ok := d.(*ast.ServiceNode); ok && string(s.Name.AsIdentifier()) == name {
			found = append(found, s)
		}
	}

	if len(found) != 1 {
		return Insertion{}, anchorError(p.path, "service "+name, len(found))
	}

	closing := p.file.NodeInfo(found[0].CloseBrace).Start().Offset

	return Insertion{Path: p.path, Offset: lineStart(p.src, closing), Text: indent(text, "  ")}, nil
}

// AtEnd appends text to the file, after a blank line.
func (p *Proto) AtEnd(text string) Insertion {
	return Insertion{Path: p.path, Offset: len(p.src), Text: "\n" + indent(text, "")}
}

// Services returns the names of the file's services, in order.
func (p *Proto) Services() []string {
	var names []string

	for _, d := range p.file.Decls {
		if s, ok := d.(*ast.ServiceNode); ok {
			names = append(names, string(s.Name.AsIdentifier()))
		}
	}

	return names
}

// Messages returns the names of the file's top-level messages, in order.
func (p *Proto) Messages() []string {
	var names []string

	for _, d := range p.file.Decls {
		if m, ok := d.(*ast.MessageNode); ok {
			names = append(names, string(m.Name.AsIdentifier()))
		}
	}

	return names
}
