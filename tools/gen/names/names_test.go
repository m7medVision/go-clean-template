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

// gqlgen and revive write the words they know as initialisms in capitals in Go names, as APIKey,
// where the generator writes ApiKey; the build or the linter would fail only after the write.
func TestParseRejectsAWordGoNamesWriteAsAnInitialism(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		kebab, plural string
		want          string
	}{
		"the first word": {"api-key", "", `"api-key" has the word "api", which gqlgen and revive write as API in Go names, ` +
			`and the generator as Api`},
		"the last word":   {"user-id", "", `"user-id" has the word "id", which gqlgen and revive write as ID`},
		"the only word":   {"url", "", `"url" has the word "url", which gqlgen and revive write as URL`},
		"before a digit":  {"http2-log", "", `"http2-log" has the word "http2", which gqlgen and revive write as HTTP2`},
		"gqlgen's own":    {"pdf-file", "", `"pdf-file" has the word "pdf", which gqlgen writes as PDF`},
		"revive's own":    {"user-ids", "", `"user-ids" has the word "ids", which revive writes as IDs`},
		"in the plural":   {"rh", "", `plural: R1 name: "rhs" has the word "rhs", which gqlgen and revive write as RHS`},
		"a plural= given": {"endpoint", "api-endpoints", `plural: R1 name: "api-endpoints" has the word "api"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := names.Parse(tc.kebab, tc.plural)

			require.ErrorIs(t, err, names.ErrInvalid)
			assert.Contains(t, err.Error(), tc.want)
			assert.Contains(t, err.Error(), "so what it generates would not build or lint: use another word")
		})
	}
}

// protoc-gen-go capitalizes a letter right after a digit in the Go names it writes, as
// Mp3PlayerResponse, and keeps the underscore before a plural's word that starts with a digit in
// the list field's, as Items_2; the generator writes Mp3playerResponse and Items2, and the gRPC
// code would fail to build only after the write.
func TestParseRejectsADigitProtocGenGoNamesDifferently(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		kebab, plural string
		want          []string
	}{
		"in the name": {"mp3player", "", []string{
			`R1 name: "mp3player" has a letter right after a digit (3p), which protoc-gen-go capitalizes in Go names ` +
				`(Mp3Player) and the generator does not (Mp3player)`,
			"put a dash after the digit, as in mp3-player",
		}},
		"in a later word": {"music-mp3player", "", []string{`(3p)`, "as in music-mp3-player"}},
		"in the plural formed from the name": {"mp3", "", []string{
			`plural: R1 name: "mp3s" has a letter right after a digit (3s)`, "(Mp3S)", "choose another plural with plural=",
		}},
		"in a plural= given": {"player", "mp3players", []string{`"mp3players" has a letter right after a digit (3p)`}},
		"starting a word of the plural": {"item", "items-2", []string{
			`plural: R1 name: "items-2" has the word "2", which starts with a digit: protoc-gen-go keeps the underscore ` +
				`before it in the Go name of the list field (Items_2)`,
			"choose another plural with plural=",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := names.Parse(tc.kebab, tc.plural)

			require.ErrorIs(t, err, names.ErrInvalid)

			for _, want := range tc.want {
				assert.Contains(t, err.Error(), want)
			}

			assert.Contains(t, err.Error(), "so the gRPC code would not build")
		})
	}
}

// The tools split a word where its letters end, so a word that ends in digits is checked by its
// letters: utf8 is Utf8 to every one of them. A word that holds an initialism is not one.
func TestParseAcceptsANameEveryToolWritesAlike(t *testing.T) {
	t.Parallel()

	for _, kebab := range []string{"idea", "utf8-text", "mp3-player", "apiary", "guide", "build-log"} {
		_, err := names.Parse(kebab, "")
		require.NoError(t, err, kebab)
	}
}
