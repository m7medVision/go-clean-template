package passkey_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportPasskeysGivesEachRowItsStream(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	used := at().Add(time.Hour)
	rows := []entity.Passkey{
		{ID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used},
		{ID: entity.PasskeyID{5}, UserID: "u1", Name: "Phone", Credential: credential(), CreatedAt: at()},
	}

	require.NoError(t, passkey.ImportPasskeys(context.Background(), store, rows))

	assert.Equal(t, []entity.Event{entity.PasskeyImported{
		PasskeyID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used,
	}}, store.Events(entity.PasskeyStreamID(id())))
	assert.Len(t, store.Events(entity.PasskeyStreamID(entity.PasskeyID{5})), 1)

	require.NoError(t, passkey.ImportPasskeys(context.Background(), store, rows), "an imported passkey is skipped")
	assert.Len(t, store.Events(entity.PasskeyStreamID(id())), 1)
}

func TestTheImporterIsThePasskeyDomains(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "passkey", passkey.NewImporter(nil, nil).Domain())
}
