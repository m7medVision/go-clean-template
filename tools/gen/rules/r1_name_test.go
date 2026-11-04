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

// authProto is the proto package grpc.v1's auth file: one service and its two messages.
const authProto = "syntax = \"proto3\";\n\npackage grpc.v1;\n\nservice AuthService {\n" +
	"  rpc Login (LoginRequest) returns (LoginResponse);\n}\n\nmessage LoginRequest {}\n\nmessage LoginResponse {}\n"

// Every .proto in one folder shares one proto package, as docs/proto/v1's share grpc.v1, so a
// message or service is taken when another file there declares one of that name. Messages and
// services share the package's names.
func TestIdentifiersFreeRejectsAProtoNameAnotherFileInTheFolderDeclares(t *testing.T) {
	t.Parallel()

	for what, tc := range map[string]struct {
		src, clash string
	}{
		"a message":            {"message LoginResponse {}\n", "LoginResponse"},
		"a service":            {"service AuthService {}\n", "AuthService"},
		"a service as message": {"message AuthService {}\n", "AuthService"},
	} {
		t.Run(what, func(t *testing.T) {
			t.Parallel()

			p := plan.New("x")
			p.Create("docs/proto/v1/login.proto", []byte("syntax = \"proto3\";\n\npackage grpc.v1;\n\nmessage LoginStarted {}\n\n"+tc.src))

			err := rules.IdentifiersFree(repoOf(fstest.MapFS{"docs/proto/v1/auth.proto": {Data: []byte(authProto)}}), name(t, "login"), p)

			require.ErrorIs(t, err, names.ErrInvalid)
			assert.Contains(t, err.Error(), "docs/proto/v1/login.proto would declare "+tc.clash+", which docs/proto/v1/auth.proto declares already")
		})
	}
}

func TestIdentifiersFreeLooksAtWhatAProtoEditAdds(t *testing.T) {
	t.Parallel()

	src := []byte(authProto)

	p := plan.New("x")
	p.Edit("docs/proto/v1/auth.proto", src, edit.Insertion{Path: "docs/proto/v1/auth.proto", Offset: len(src), Text: "\nmessage LoginRequest {}\n"})

	err := rules.IdentifiersFree(repoOf(fstest.MapFS{"docs/proto/v1/auth.proto": {Data: src}}), name(t, "login"), p)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "docs/proto/v1/auth.proto would declare LoginRequest twice")
}

func TestIdentifiersFreeAcceptsProtoNamesThatAreNewToTheirFolder(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"docs/proto/v1/auth.proto": {Data: []byte(authProto)}}
	src := fsys["docs/proto/v1/auth.proto"].Data

	p := plan.New("x")
	p.Create("docs/proto/v1/invoice.proto", []byte("syntax = \"proto3\";\n\npackage grpc.v1;\n\nservice InvoiceService {}\n\nmessage InvoiceResponse {}\n"))
	p.Create("docs/proto/v2/login.proto", []byte("syntax = \"proto3\";\n\npackage grpc.v2;\n\nservice AuthService {}\n\nmessage LoginResponse {}\n"))
	p.Edit("docs/proto/v1/auth.proto", src, edit.Insertion{Path: "docs/proto/v1/auth.proto", Offset: len(src), Text: "\nmessage LogoutRequest {}\n"})

	require.NoError(t, rules.IdentifiersFree(repoOf(fsys), name(t, "invoice"), p),
		"names new to docs/proto/v1 are free, and docs/proto/v2 is a package of its own")
}
