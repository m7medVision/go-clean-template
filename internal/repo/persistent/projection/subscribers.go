package projection

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

var (
	// ErrDuplicateSubscriber is two subscribers with one name: they would share a checkpoint.
	ErrDuplicateSubscriber = errors.New("projection: two subscribers share a name")
	// ErrReservedSubscriber is a name starting with an underscore, which the backfill's markers use.
	ErrReservedSubscriber = errors.New("projection: a subscriber name may not start with an underscore")
	// ErrUnprojectedEvent is an event type no projector handles: it would reach no read model.
	ErrUnprojectedEvent = errors.New("projection: an event type no projector handles")
)

// Names lists the projectors' names: what a write waits for.
func Names(projectors []Projector) []string {
	names := make([]string, len(projectors))
	for i, p := range projectors {
		names[i] = p.Name()
	}

	return names
}

// Check refuses subscribers the runner cannot run together: two with one name, a name the backfill's
// markers reserve, and an event type among events that no projector handles.
func Check(projectors []Projector, handlers []repo.Subscriber, events []entity.Event) error {
	subs := make([]repo.Subscriber, 0, len(projectors)+len(handlers))
	for _, p := range projectors {
		subs = append(subs, p)
	}

	seen := make(map[string]bool, len(subs))

	for _, s := range append(subs, handlers...) {
		switch {
		case strings.HasPrefix(s.Name(), "_"):
			return fmt.Errorf("%w: %q", ErrReservedSubscriber, s.Name())
		case seen[s.Name()]:
			return fmt.Errorf("%w: %q", ErrDuplicateSubscriber, s.Name())
		}

		seen[s.Name()] = true
	}

	for _, e := range events {
		if !slices.ContainsFunc(projectors, func(p Projector) bool { return handles(p, e) }) {
			return fmt.Errorf("%w: %s", ErrUnprojectedEvent, e.EventType())
		}
	}

	return nil
}

func handles(s repo.Subscriber, e entity.Event) bool {
	return slices.ContainsFunc(s.Handles(), func(h entity.Event) bool { return h.EventType() == e.EventType() })
}
