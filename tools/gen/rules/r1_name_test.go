package rules_test

import (
	"go/token"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entityPackage is an entity package with a function, a sentinel block, a method, an external test
// package and a pair of files that declare one name under opposite build tags.
func entityPackage() fstest.MapFS {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

	return fstest.MapFS{
		"internal/entity/aggregate.go":      file("package entity\n\nfunc Record() {}\n\ntype Base struct{}\n\nfunc (*Base) Version() int { return 0 }\n"),
		"internal/entity/errors.go":         file("package entity\n\nimport \"errors\"\n\nvar (\n\tErrA = errors.New(\"a\")\n)\n"),
		"internal/entity/aggregate_test.go": file("package entity_test\n\nfunc given() {}\n"),
		"internal/entity/on.go":             file("//go:build on\n\npackage entity\n\nfunc mode() {}\n"),
		"internal/entity/off.go":            file("//go:build !on\n\npackage entity\n\nfunc mode() {}\n"),
	}
}

func TestIdentifiersFreeRejectsADeclarationThePackageHas(t *testing.T) {
	t.Parallel()

	for what, tc := range map[string]struct {
		path, src string
		clash     string
	}{
		"a function":   {"internal/entity/record.go", "package entity\n\ntype Record struct{}\n", "Record"},
		"a method":     {"internal/entity/base.go", "package entity\n\nfunc (Base) Version() int { return 1 }\n", "Base.Version"},
		"a test name":  {"internal/entity/record_test.go", "package entity_test\n\nfunc given() {}\n", "given"},
		"a build pair": {"internal/entity/mode.go", "package entity\n\nconst mode = 1\n", "mode"},
	} {
		t.Run(what, func(t *testing.T) {
			t.Parallel()

			p := plan.New("x")
			p.Create(tc.path, []byte(tc.src))

			err := rules.IdentifiersFree(repoOf(entityPackage()), name(t, "record"), p)

			require.ErrorIs(t, err, names.ErrInvalid)
			assert.Contains(t, err.Error(), tc.path+" would declare "+tc.clash+",")
		})
	}
}

func TestIdentifiersFreeLooksAtWhatAnEditAdds(t *testing.T) {
	t.Parallel()

	fsys := entityPackage()
	src := fsys["internal/entity/errors.go"].Data

	g, err := edit.ParseGo("internal/entity/errors.go", src)
	require.NoError(t, err)

	ins, err := g.BeforeBlockEnd(token.VAR, "ErrA = errors.New(\"again\")")
	require.NoError(t, err)

	p := plan.New("x")
	p.Edit("internal/entity/errors.go", src, ins)

	err = rules.IdentifiersFree(repoOf(fsys), name(t, "record"), p)
	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "internal/entity/errors.go would declare ErrA twice")
}

func TestIdentifiersFreeAcceptsDeclarationsThatAreNew(t *testing.T) {
	t.Parallel()

	fsys := entityPackage()
	src := fsys["internal/entity/errors.go"].Data

	g, err := edit.ParseGo("internal/entity/errors.go", src)
	require.NoError(t, err)

	ins, err := g.BeforeBlockEnd(token.VAR, "ErrInvoice = errors.New(\"invoice\")")
	require.NoError(t, err)

	p := plan.New("x")
	p.Edit("internal/entity/errors.go", src, ins)
	p.Create("internal/entity/invoice.go", []byte("package entity\n\ntype Invoice struct{}\n\nfunc (Invoice) Version() int { return 1 }\n"))
	p.Create("internal/entity/invoice_test.go", []byte("package entity_test\n\nfunc givenInvoice() {}\n\nfunc Record() {}\n"))
	p.Create("internal/usecase/invoice/register.go", []byte("package invoice\n\nfunc Register() {}\n"))
	p.Create("internal/repo/persistent/queries/invoice.sql", []byte("-- name: GetInvoice :one\n"))

	require.NoError(t, rules.IdentifiersFree(repoOf(fsys), name(t, "invoice"), p),
		"a name the external test package declares is not the package's, and the build pair was there before")
}
