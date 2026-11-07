package passkey_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rows is a webauthn_credentials table holding legacy rows; any other query panics on the nil
// embedded interface.
type rows struct {
	queries.Querier

	passkeys []queries.WebauthnCredential
	err      error
}

func (r rows) ListEveryPasskey(context.Context) ([]queries.WebauthnCredential, error) {
	return r.passkeys, r.err
}

func TestEachPasskeyRowBecomesAStreamHoldingPasskeyImported(t *testing.T) {
	t.Parallel()

	used := at().Add(time.Hour)

	imports, err := passkey.Imports(context.Background(), rows{passkeys: []queries.WebauthnCredential{
		{ID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used},
		{ID: entity.PasskeyID{5}, UserID: "u1", Name: "Phone", Credential: credential(), CreatedAt: at()},
	}})
	require.NoError(t, err)
	require.Len(t, imports, 2)

	assert.Equal(t, entity.PasskeyStreamID(id()), imports[0].Aggregate.StreamID())
	assert.Equal(t, []entity.Event{entity.PasskeyImported{
		PasskeyID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used,
	}}, imports[0].Aggregate.Pending())
	assert.Equal(t, entity.PasskeyStreamID(entity.PasskeyID{5}), imports[1].Aggregate.StreamID())
}

func TestThePasskeyImporterReturnsAReadFailure(t *testing.T) {
	t.Parallel()

	_, err := passkey.Imports(context.Background(), rows{err: errWrite})

	require.ErrorIs(t, err, errWrite)
}
