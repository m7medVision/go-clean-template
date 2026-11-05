package edit

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// YAML is a parsed YAML file that changes are planned against.
type YAML struct {
	path string
	src  []byte
	root *yaml.Node
}

// ParseYAML parses src, the content of the YAML file at path.
func ParseYAML(path string, src []byte) (*YAML, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &YAML{path: path, src: src, root: &root}, nil
}

// InMap inserts the entry name, holding text, as the last entry of the block mapping under the
// top-level key, at its entries' indentation. The mapping must not hold name already.
func (y *YAML) InMap(key, name, text string) (Insertion, error) {
	m, err := y.blockMap(key)
	if err != nil {
		return Insertion{}, err
	}

	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			return Insertion{}, fmt.Errorf("%w: %s: %s already holds %s", ErrAnchor, y.path, key, name)
		}
	}

	entryIndent := strings.Repeat(" ", m.Content[0].Column-1)

	return Insertion{
		Path:   y.path,
		Offset: y.blockEnd(m.Content[0].Line, len(entryIndent)),
		Text:   indent(name+":\n"+indent(text, "  "), entryIndent),
	}, nil
}

// blockMap returns the non-empty block mapping under the top-level key.
func (y *YAML) blockMap(key string) (*yaml.Node, error) {
	if len(y.root.Content) == 1 && y.root.Content[0].Kind == yaml.MappingNode {
		doc := y.root.Content[0]
		for i := 0; i+1 < len(doc.Content); i += 2 {
			v := doc.Content[i+1]
			if doc.Content[i].Value == key && v.Kind == yaml.MappingNode && v.Style&yaml.FlowStyle == 0 && len(v.Content) > 0 {
				return v, nil
			}
		}
	}

	return nil, anchorError(y.path, "a block mapping under "+key, 0)
}

// blockEnd is the offset just after the last non-blank line of the block whose first entry is on
// line first and whose entries are indented by width. The block ends at the first later line that
// is indented less, comments included.
func (y *YAML) blockEnd(first, width int) int {
	offset, end := 0, 0

	for i, l := range strings.SplitAfter(string(y.src), "\n") {
		offset += len(l)

		if i+1 < first || strings.TrimSpace(l) == "" {
			continue
		}

		if len(l)-len(strings.TrimLeft(l, " ")) < width {
			break
		}

		end = offset
	}

	return end
}
