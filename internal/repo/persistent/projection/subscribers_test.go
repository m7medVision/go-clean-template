package projection_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/stretchr/testify/require"
)

func TestCheckAcceptsSubscribersThatProjectEveryEvent(t *testing.T) {
	t.Parallel()

	require.NoError(t, projection.Check(
		[]projection.Projector{&projector{recorder{name: "a"}}},
		[]repo.Subscriber{&recorder{name: "b"}},
		[]entity.Event{ticked{}},
	))
}

func TestCheckRefusesTwoSubscribersWithOneName(t *testing.T) {
	t.Parallel()

	err := projection.Check(
		[]projection.Projector{&projector{recorder{name: "a"}}}, []repo.Subscriber{&recorder{name: "a"}}, nil)

	require.ErrorIs(t, err, projection.ErrDuplicateSubscriber)
}

// An event only an event handler hears would reach no read model.
func TestCheckRefusesAnEventNoProjectorHandles(t *testing.T) {
	t.Parallel()

	err := projection.Check(
		[]projection.Projector{&projector{recorder{name: "a"}}},
		[]repo.Subscriber{&recorder{name: "b", handles: []entity.Event{tocked{}}}},
		[]entity.Event{ticked{}, tocked{}},
	)

	require.ErrorIs(t, err, projection.ErrUnprojectedEvent)
}
