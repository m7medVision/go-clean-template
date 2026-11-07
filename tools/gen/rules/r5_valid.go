package rules

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"path"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
)

// valid checks R5: every file the plan creates or edits parses afterwards, and every Go file it
// creates is already formatted. An unformatted created file means its template is wrong; edited
// Go files are formatted by the plan.
func valid(p *plan.Plan) error {
	changes, err := p.Changes()
	if err != nil {
		return err
	}

	for _, c := range changes {
		if err := parses(c.Path, c.New); err != nil {
			return fmt.Errorf("%w: %s: %w", plan.ErrInvalid, c.Path, err)
		}

		if c.Old == nil && path.Ext(c.Path) == ".go" {
			if formatted, err := format.Source(c.New); err != nil || !bytes.Equal(formatted, c.New) {
				return fmt.Errorf("%w: %s is not go/format clean: its template is wrong", plan.ErrInvalid, c.Path)
			}
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
