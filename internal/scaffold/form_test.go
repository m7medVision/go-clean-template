package scaffold_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChoicesOfferEveryOptionalFeatureWithItsRequirements(t *testing.T) {
	t.Parallel()

	got := fixture(t).Choices()

	assert.Equal(t, []scaffold.Choice{
		{ID: "graphql", Label: "GraphQL"},
		{ID: "web", Label: "Frontend (needs GraphQL)"},
		{ID: "sso", Label: "SSO (needs Frontend)"},
		{ID: "grpc", Label: "gRPC"},
	}, got, "core features are not offered: they are always included")
}

func TestPresetFeaturesPrefillTheChoices(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	got, err := m.PresetFeatures("fullstack")
	require.NoError(t, err)
	assert.Equal(t, []string{"graphql", "web"}, got)

	got, err = m.PresetFeatures(scaffold.CustomPreset)
	require.NoError(t, err)
	assert.Empty(t, got, "custom starts from the core features alone")

	_, err = m.PresetFeatures("nope")
	require.ErrorIs(t, err, scaffold.ErrInvalidSelection)
}

func TestValidateChoicesNamesEveryMissingRequirement(t *testing.T) {
	t.Parallel()

	m := fixture(t)

	require.NoError(t, m.ValidateChoices([]string{"graphql", "web"}))

	err := m.ValidateChoices([]string{"web", "sso"})
	require.ErrorIs(t, err, scaffold.ErrInvalidSelection)
	assert.Equal(t, "web needs graphql", scaffold.Reason(err))
}

func TestPreviewCountsWithoutWriting(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_generateManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{Without: []string{"grpc"}})
	require.NoError(t, err)

	_, files := template(t)

	assert.Equal(t, scaffold.Report{Kept: 6, Removed: 3}, m.Preview(keep, files))
}
