package app

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSubscriber struct {
	name   string
	tables []string
}

func (f fakeSubscriber) Name() string { return f.name }

func (fakeSubscriber) Handle(context.Context, *entity.RecordedEvent) error { return nil }

type fakeProjector struct{ fakeSubscriber }

func (f fakeProjector) Tables() []string { return f.tables }

func TestProjectorNamesListOnlyProjectors(t *testing.T) {
	t.Parallel()

	subs := []repo.Subscriber{
		fakeProjector{fakeSubscriber{name: "task", tables: []string{"tasks"}}},
		fakeSubscriber{name: "send-welcome-email"},
		fakeProjector{fakeSubscriber{name: "user", tables: []string{"users"}}},
	}

	assert.Equal(t, []string{"task", "user"}, projectorNames(subs))
}

func TestCheckSubscribersRejectsADuplicateName(t *testing.T) {
	t.Parallel()

	require.NoError(t, checkSubscribers([]repo.Subscriber{fakeSubscriber{name: "a"}, fakeSubscriber{name: "b"}}))

	err := checkSubscribers([]repo.Subscriber{fakeSubscriber{name: "a"}, fakeSubscriber{name: "a"}})
	require.ErrorIs(t, err, errDuplicateSubscriber)
}

func TestCheckSubscribersRejectsAMarkerName(t *testing.T) {
	t.Parallel()

	err := checkSubscribers([]repo.Subscriber{fakeSubscriber{name: "_backfill/task"}})

	require.ErrorIs(t, err, errReservedSubscriber)
}
