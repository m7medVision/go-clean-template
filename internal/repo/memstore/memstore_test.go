package memstore_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventstoretest"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemstorePassesTheEventStoreSuite(t *testing.T) {
	t.Parallel()

	eventstoretest.Run(t, func(*testing.T) repo.EventStore {
		return memstore.New(eventstoretest.Events(), memstore.SnapshotEvery(eventstoretest.SnapshotEvery))
	})
}

func TestEventsReturnsAStreamsEventsInOrder(t *testing.T) {
	t.Parallel()

	s := memstore.New(eventstoretest.Events())
	n := eventstoretest.NewNote("n1", "first")
	n.Edit("second")

	_, err := s.Save(context.Background(), n)
	require.NoError(t, err)

	assert.Equal(t, []entity.Event{
		eventstoretest.NoteWritten{ID: "n1", Text: "first"},
		eventstoretest.NoteEdited{Text: "second"},
	}, s.Events(n.StreamID()))
	assert.Empty(t, s.Events("missing"))
}
