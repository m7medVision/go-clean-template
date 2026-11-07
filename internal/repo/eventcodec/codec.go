// Package eventcodec holds what every event store shares, whatever it stores in: the codec that turns
// events into stored rows and back (upcasting old versions), when a stream is due a snapshot, what a
// taken claim answers, and the metadata an event is stored with. It imports no driver, so the
// Postgres store and the in-memory one both use it.
package eventcodec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"go.opentelemetry.io/otel/trace"
)

var (
	// ErrUnknownEvent is an event type the codec was not given.
	ErrUnknownEvent = errors.New("eventcodec: unknown event type")
	// ErrNoUpcaster is a stored version with no upcaster to the next one.
	ErrNoUpcaster = errors.New("eventcodec: no upcaster")
	// ErrNewerVersion is a stored version newer than the code: a binary older than the data.
	ErrNewerVersion = errors.New("eventcodec: event version newer than the code")
)

// Stored is an event as a row holds it.
type Stored struct {
	Type          string
	SchemaVersion int
	Payload       json.RawMessage
}

type upcastKey struct {
	eventType string
	from      int
}

// Codec turns events into stored rows and back, upcasting old versions on the way in.
type Codec struct {
	types     map[string]reflect.Type
	versions  map[string]int
	upcasters map[upcastKey]entity.Upcast
}

// New returns a codec for events (one zero value per type, as entity.EventTypes lists them) and the
// upcasts between their stored versions.
func New(events []entity.Event, upcasts ...entity.Upcast) *Codec {
	c := &Codec{
		types:     make(map[string]reflect.Type, len(events)),
		versions:  make(map[string]int, len(events)),
		upcasters: make(map[upcastKey]entity.Upcast, len(upcasts)),
	}

	for _, e := range events {
		c.types[e.EventType()] = reflect.TypeOf(e)
		c.versions[e.EventType()] = e.SchemaVersion()
	}

	for _, u := range upcasts {
		c.upcasters[upcastKey{eventType: u.Type, from: u.From}] = u
	}

	return c
}

// Encode returns the row for e at its current version.
func (c *Codec) Encode(e entity.Event) (Stored, error) {
	if _, ok := c.types[e.EventType()]; !ok {
		return Stored{}, fmt.Errorf("%w: %s", ErrUnknownEvent, e.EventType())
	}

	payload, err := json.Marshal(e)
	if err != nil {
		return Stored{}, fmt.Errorf("eventcodec - Encode - json.Marshal %s: %w", e.EventType(), err)
	}

	return Stored{Type: e.EventType(), SchemaVersion: e.SchemaVersion(), Payload: payload}, nil
}

// Decode returns the event a row holds, upcast one version at a time to its current version.
func (c *Codec) Decode(s Stored) (entity.Event, error) {
	t, ok := c.types[s.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEvent, s.Type)
	}

	current := c.versions[s.Type]
	if s.SchemaVersion > current {
		return nil, fmt.Errorf("%w: %s v%d, the code knows v%d", ErrNewerVersion, s.Type, s.SchemaVersion, current)
	}

	payload := s.Payload

	for version := s.SchemaVersion; version < current; version++ {
		u, found := c.upcasters[upcastKey{eventType: s.Type, from: version}]
		if !found {
			return nil, fmt.Errorf("%w: %s v%d to v%d", ErrNoUpcaster, s.Type, version, version+1)
		}

		var err error
		if payload, err = u.Up(payload); err != nil {
			return nil, fmt.Errorf("eventcodec - Decode - upcast %s v%d: %w", s.Type, version, err)
		}
	}

	// A new value of the event's concrete type, filled from the payload.
	v := reflect.New(t)
	if err := json.Unmarshal(payload, v.Interface()); err != nil {
		return nil, fmt.Errorf("eventcodec - Decode - json.Unmarshal %s: %w", s.Type, err)
	}

	e, ok := v.Elem().Interface().(entity.Event)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEvent, s.Type)
	}

	return e, nil
}

// ClaimTaken is the error for claim c, held by another owner: its own Taken error, or ErrClaimTaken.
func ClaimTaken(c *entity.Claim) error {
	if c.Taken != nil {
		return c.Taken
	}

	return fmt.Errorf("%w: %s %q", entity.ErrClaimTaken, c.Scope, c.Value)
}

// Metadata is the metadata an event is stored with: the context's, with the trace it runs in as
// the correlation when it has none.
func Metadata(ctx context.Context) entity.Metadata {
	md := entity.MetadataFromContext(ctx)

	if sc := trace.SpanContextFromContext(ctx); md.CorrelationID == "" && sc.HasTraceID() {
		md.CorrelationID = sc.TraceID().String()
	}

	return md
}
