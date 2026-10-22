package scaffold_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rules keeps rest and graphql; grpc and web are known but dropped.
func rules(t *testing.T) scaffold.Rules {
	t.Helper()

	m := fixture(t)

	set, err := m.Resolve(scaffold.Selection{With: []string{"graphql"}})
	require.NoError(t, err)

	return scaffold.Rules{Manifest: m, Keep: set}
}

type stripCase struct {
	name, file, in, want string
}

func runStripCases(t *testing.T, tests []stripCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := rules(t).Strip(tt.file, []byte(tt.in))
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestStripCommentSyntaxes(t *testing.T) {
	t.Parallel()

	runStripCases(t, []stripCase{
		{"slash comments", "main.ts", "a\n// scaffold:grpc:begin\nb\n// scaffold:grpc:end\nc\n", "a\nc\n"},
		{"makefile", "Makefile", "a\n# scaffold:grpc:begin\nproto:\n\tprotoc\n# scaffold:grpc:end\nc\n", "a\nc\n"},
		{"yaml", "docker-compose.yml", "a:\n  # scaffold:grpc:begin\n  - 8081\n  # scaffold:grpc:end\n  - 8080\n", "a:\n  - 8080\n"},
		{"dotenv", ".env.example", "A=1\n# scaffold:grpc:begin\nGRPC_PORT=8081\n# scaffold:grpc:end\n", "A=1\n"},
		{"markdown", "docs/api.md", "a\n<!-- scaffold:grpc:begin -->\n## gRPC\n<!-- scaffold:grpc:end -->\nc\n", "a\nc\n"},
		{"sql", "queries/task.sql", "a\n-- scaffold:grpc:begin\nb\n-- scaffold:grpc:end\nc\n", "a\nc\n"},
		{"typescript", "web/src/main.tsx", "a\n// scaffold:grpc:begin\nb\n// scaffold:grpc:end\nc\n", "a\nc\n"},
	})
}

func TestStripFeatureRules(t *testing.T) {
	t.Parallel()

	runStripCases(t, []stripCase{
		{"kept block loses only its markers", "main.ts", "a\n\t// scaffold:graphql:begin\n\tb\n\t// scaffold:graphql:end\nc\n", "a\n\tb\nc\n"},
		{"several features: kept only when all are", "main.ts", "a\n// scaffold:graphql,grpc:begin\nb\n// scaffold:graphql,grpc:end\nc\n", "a\nc\n"},
		{"several features, all kept", "main.ts", "a\n// scaffold:rest,graphql:begin\nb\n// scaffold:rest,graphql:end\nc\n", "a\nb\nc\n"},
		{"nested: a dropped outer block drops the inner one", "main.ts", "a\n// scaffold:grpc:begin\nb\n// scaffold:graphql:begin\nc\n// scaffold:graphql:end\n// scaffold:grpc:end\nd\n", "a\nd\n"},
		{"nested: a kept outer block keeps a kept inner one", "main.ts", "a\n// scaffold:graphql:begin\nb\n// scaffold:grpc:begin\nc\n// scaffold:grpc:end\n// scaffold:graphql:end\nd\n", "a\nb\nd\n"},
		{"self is never kept", "Makefile", "a\n# scaffold:self:begin\nscaffold:\n# scaffold:self:end\nc\n", "a\nc\n"},
	})
}

func TestStripTrailingMarkers(t *testing.T) {
	t.Parallel()

	runStripCases(t, []stripCase{
		{"trailing marker drops one line", "main.ts", "a\n\trestapi.WithOIDC(oc), // scaffold:grpc\nc\n", "a\nc\n"},
		{"trailing marker on a kept line is removed", "main.ts", "a\n\tmountGraphQL(router) // scaffold:graphql\nc\n", "a\n\tmountGraphQL(router)\nc\n"},
		{"trailing markdown marker", "README.md", "| REST | yes |\n| gRPC | yes | <!-- scaffold:grpc -->\n", "| REST | yes |\n"},
		{"a dropped block between blank lines leaves one", "docs/api.md", "a\n\n<!-- scaffold:grpc:begin -->\n## gRPC\n\ntext\n<!-- scaffold:grpc:end -->\n\nc\n", "a\n\nc\n"},
	})
}

func TestStripLeavesTextAlone(t *testing.T) {
	t.Parallel()

	runStripCases(t, []stripCase{
		{"a file with no markers is unchanged", "README.md", "just text\n", "just text\n"},
		{"a marker-looking phrase inside a sentence is text", "CLAUDE.md", "write `// scaffold:grpc:begin` above the block\n", "write `// scaffold:grpc:begin` above the block\n"},
		{"another file type's comment syntax is text", "README.md", "# scaffold:grpc:begin\n", "# scaffold:grpc:begin\n"},
	})
}

func TestStripRejectsBrokenMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file, in, wantErr string
	}{
		{"unclosed", "app.go", "a\n// scaffold:grpc:begin\nb\n", "app.go:2: scaffold:grpc:begin is never closed"},
		{"end without begin", "app.go", "a\n// scaffold:grpc:end\n", "app.go:2: scaffold:grpc:end without a begin"},
		{"mismatched end", "app.go", "// scaffold:grpc:begin\n// scaffold:web:end\n", "app.go:2: scaffold:web:end closes scaffold:grpc:begin"},
		{"unknown feature", "app.go", "// scaffold:nope:begin\n// scaffold:nope:end\n", `app.go:1: unknown feature "nope"`},
		{"unknown feature in a trailing marker", "app.go", "x // scaffold:nope\n", `app.go:1: unknown feature "nope"`},
		{"markers in a file with no comment syntax", "realm.json", "\"// scaffold:grpc:begin\"\n", "realm.json: markers need a file type with comments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := rules(t).Strip(tt.file, []byte(tt.in))
			require.ErrorIs(t, err, scaffold.ErrBadMarker)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestStripRemovesGoImportsTheDroppedCodeUsed(t *testing.T) {
	t.Parallel()

	in := `package app

import (
	"fmt"
	"strings"
)

func f() {
	// scaffold:grpc:begin
	fmt.Println("grpc")
	// scaffold:grpc:end
	_ = strings.TrimSpace(" ")
}
`
	want := `package app

import (
	"strings"
)

func f() {
	_ = strings.TrimSpace(" ")
}
`

	got, err := rules(t).Strip("internal/app/app.go", []byte(in))
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
