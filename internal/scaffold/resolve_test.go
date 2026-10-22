package scaffold_test

import (
	"path/filepath"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _manifest = `
module: example.com/template
features:
  - id: rest
    label: REST
    core: true
  - id: graphql
    label: GraphQL
  - id: web
    label: Frontend
    requires: [graphql]
  - id: sso
    label: SSO
    requires: [web]
  - id: grpc
    label: gRPC
presets:
  - id: api
    label: API only
    features: []
  - id: fullstack
    label: Fullstack
    features: [graphql, web]
`

func fixture(t *testing.T) *scaffold.Manifest {
	t.Helper()

	m, err := scaffold.Parse([]byte(_manifest))
	require.NoError(t, err)

	return m
}

func TestParseRejectsInvalidManifests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, doc, wantErr string
	}{
		{"unknown requirement", "features:\n  - {id: web, requires: [graphql]}\n", `feature "web" requires unknown feature "graphql"`},
		{"duplicate id", "features:\n  - {id: web}\n  - {id: web}\n", `feature "web" is declared twice`},
		{"reserved id", "features:\n  - {id: self}\n", `feature id "self" is reserved`},
		{"cycle", "features:\n  - {id: a, requires: [b]}\n  - {id: b, requires: [a]}\n", "requirement cycle"},
		{"preset with unknown feature", "features:\n  - {id: a}\npresets:\n  - {id: p, features: [b]}\n", `preset "p" lists unknown feature "b"`},
		{"preset breaking a rule", "features:\n  - {id: a}\n  - {id: b, requires: [a]}\npresets:\n  - {id: p, features: [b]}\n", `preset "p": b needs a`},
		{"not yaml", "features: [", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := scaffold.Parse([]byte(tt.doc))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	tests := []struct {
		name string
		sel  scaffold.Selection
		want []string
	}{
		{"preset", scaffold.Selection{Preset: "fullstack"}, []string{"rest", "graphql", "web"}},
		{"empty preset keeps core", scaffold.Selection{Preset: "api"}, []string{"rest"}},
		{"no preset means everything", scaffold.Selection{}, []string{"rest", "graphql", "web", "sso", "grpc"}},
		{"without from everything", scaffold.Selection{Without: []string{"grpc"}}, []string{"rest", "graphql", "web", "sso"}},
		{"with alone starts from core", scaffold.Selection{With: []string{"grpc"}}, []string{"rest", "grpc"}},
		{"preset plus with", scaffold.Selection{Preset: "api", With: []string{"grpc"}}, []string{"rest", "grpc"}},
		{"preset minus without", scaffold.Selection{Preset: "fullstack", Without: []string{"web"}}, []string{"rest", "graphql"}},
		{"custom preset starts from core", scaffold.Selection{Preset: scaffold.CustomPreset}, []string{"rest"}},
		{"custom preset plus with", scaffold.Selection{Preset: scaffold.CustomPreset, With: []string{"grpc"}}, []string{"rest", "grpc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := m.Resolve(tt.sel)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got.IDs())
		})
	}
}

func TestResolveRejects(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	tests := []struct {
		name    string
		sel     scaffold.Selection
		wantErr string
	}{
		{"missing requirement", scaffold.Selection{Preset: "fullstack", Without: []string{"graphql"}}, "web needs graphql"},
		{"every missing requirement", scaffold.Selection{Without: []string{"graphql", "web"}}, "sso needs web"},
		{"unknown preset", scaffold.Selection{Preset: "nope"}, `unknown preset "nope"`},
		{"unknown feature", scaffold.Selection{With: []string{"nope"}}, `unknown feature "nope"`},
		{"dropping core", scaffold.Selection{Without: []string{"rest"}}, `"rest" is always included`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := m.Resolve(tt.sel)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestProblemsNamesEveryMissingRequirement(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	assert.Equal(t, []string{"web needs graphql"}, m.Problems([]string{"web", "sso"}))
	assert.Equal(t, []string{"sso needs web"}, m.Problems([]string{"graphql", "sso"}))
	assert.Empty(t, m.Problems([]string{"graphql", "web", "sso"}))
}

func TestSetKeepsCoreAndNeverSelf(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	set, err := m.Resolve(scaffold.Selection{Preset: "api"})
	require.NoError(t, err)

	assert.True(t, set.Has("rest"))
	assert.False(t, set.Has("grpc"))
	assert.False(t, set.Has(scaffold.SelfFeature), "the scaffolder never ships")
}

func TestRepoManifestIsValidAndEveryPresetResolves(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Load(filepath.Join("..", "..", scaffold.ManifestPath))
	require.NoError(t, err)

	for _, p := range m.Presets {
		_, err := m.Resolve(scaffold.Selection{Preset: p.ID})
		require.NoError(t, err, p.ID)
	}
}

func TestDependentsListsEveryFeatureThatNeedsOneTransitively(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	assert.Equal(t, []string{"web", "sso"}, m.Dependents("graphql"))
	assert.Equal(t, []string{"sso"}, m.Dependents("web"))
	assert.Empty(t, m.Dependents("grpc"))
}
