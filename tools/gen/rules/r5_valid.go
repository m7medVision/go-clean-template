package rules

import (
	"fmt"
	"go/parser"
	"go/token"
	"path"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
)

// valid checks R5: every file the plan creates or edits parses afterwards.
func valid(p *plan.Plan) error {
	changes, err := p.Changes()
	if err != nil {
		return err
	}

	for _, c := range changes {
		if err := parses(c.Path, c.New); err != nil {
			return fmt.Errorf("%w: %s: %w", plan.ErrInvalid, c.Path, err)
		}
	}

	return nil
}

// parses parses src by the kind of file at p. Kinds without a parser here, such as SQL and HCL,
// are checked by the regenerators that read them.
func parses(p string, src []byte) error {
	var err error

	switch path.Ext(p) {
	case ".go":
		_, err = parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	case ".proto":
		_, err = edit.ParseProto(p, src)
	case ".graphqls":
		_, err = edit.ParseGraphQL(p, src)
	}

	return err
}
