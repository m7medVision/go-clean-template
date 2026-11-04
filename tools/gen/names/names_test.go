package names_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDerivesEveryForm(t *testing.T) {
	t.Parallel()

	n, err := names.Parse("line-item", "")
	require.NoError(t, err)

	assert.Equal(t, names.Name{
		Forms: names.Forms{
			Kebab: "line-item", Go: "LineItem", Lower: "lineItem", Pkg: "lineitem", Snake: "line_item", Words: "line item",
		},
		Plural: names.Forms{
			Kebab: "line-items", Go: "LineItems", Lower: "lineItems", Pkg: "lineitems", Snake: "line_items",
			Words: "line items",
		},
	}, n)
}

func TestParsePluralizesTheLastWord(t *testing.T) {
	t.Parallel()

	for singular, plural := range map[string]string{
		"invoice":    "invoices",
		"task":       "tasks",
		"address":    "addresses",
		"box":        "boxes",
		"buzz":       "buzzes",
		"branch":     "branches",
		"wish":       "wishes",
		"category":   "categories",
		"day":        "days",
		"line-item":  "line-items",
		"sales-tax":  "sales-taxes",
		"key2":       "key2s",
		"user-story": "user-stories",
	} {
		n, err := names.Parse(singular, "")
		require.NoError(t, err, singular)
		assert.Equal(t, plural, n.Plural.Kebab, singular)
	}
}

func TestParseTakesThePluralOverride(t *testing.T) {
	t.Parallel()

	n, err := names.Parse("person", "people")
	require.NoError(t, err)

	assert.Equal(t, "People", n.Plural.Go)
	assert.Equal(t, "people", n.Plural.Snake)
}

func TestParseRejectsABadName(t *testing.T) {
	t.Parallel()

	for name, in := range map[string][2]string{
		"empty":                {"", ""},
		"upper case":           {"Invoice", ""},
		"snake case":           {"line_item", ""},
		"leading digit":        {"2fa", ""},
		"trailing hyphen":      {"invoice-", ""},
		"double hyphen":        {"line--item", ""},
		"go keyword":           {"type", ""},
		"predeclared":          {"string", ""},
		"keyword as camel":     {"go-to", ""},
		"imported package":     {"query", ""},
		"plural same as name":  {"sheep", "sheep"},
		"bad plural":           {"person", "People"},
		"plural is a keyword":  {"rang", "range"},
		"generated api prefix": {"v1", ""},
	} {
		_, err := names.Parse(in[0], in[1])
		require.ErrorIs(t, err, names.ErrInvalid, name)
	}
}
