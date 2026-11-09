package mongo_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/mongo"
	"github.com/stretchr/testify/require"
)

func TestNewFails(t *testing.T) {
	t.Parallel()

	for name, url := range map[string]string{
		"an unparsable URL":             "mongo://nowhere",
		"an address nothing listens on": "mongodb://127.0.0.1:1/app?serverSelectionTimeoutMS=200&connectTimeoutMS=200",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, err := mongo.New(t.Context(), url)
			require.Error(t, err)
		})
	}
}

func TestNewNeedsADatabase(t *testing.T) {
	t.Parallel()

	_, _, err := mongo.New(t.Context(), "mongodb://127.0.0.1:1/")
	require.ErrorIs(t, err, mongo.ErrNoDatabase)
}
