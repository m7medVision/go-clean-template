// Package memstore is an in-memory repo.EventStore for tests. It shares internal/repo/eventcodec with
// the Postgres store (the codec, snapshots, claims and metadata) and passes the same contract suite
// (internal/repo/eventstoretest), so a command handler tested against it behaves the same against
// Postgres.
package memstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
)

var _ repo.EventStore = (*Store)(nil)

// Option configures a Store.
type Option func(*Store)

// SnapshotEvery makes the store snapshot a stream each time it crosses a multiple of n events.
func SnapshotEvery(n int) Option {
	return func(s *Store) { s.snapshotEvery = n }
}

type row struct {
	position   int64
	version    int
	stored     eventcodec.Stored
	metadata   entity.Metadata
	recordedAt time.Time
}

type snapshot struct {
	version       int
	schemaVersion int
	payload       json.RawMessage
}

type claimKey struct {
	scope string
	value string
}

// Store keeps every stream in memory.
type Store struct {
	mu            sync.Mutex
	codec         *eventcodec.Codec
	snapshotEvery int
	position      int64
	streams       map[string][]row
	snapshots     map[string]snapshot
	claims        map[claimKey]string
}

// New returns an empty store that knows events (entity.EventTypes() for the domain's own), upcasting
// them with the domain's entity.Upcasts as the app's store does.
func New(events []entity.Event, opts ...Option) *Store {
	s := &Store{
		codec:     eventcodec.New(events, entity.Upcasts()...),
		streams:   map[string][]row{},
		snapshots: map[string]snapshot{},
		claims:    map[claimKey]string{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Save appends a's pending events and applies claims, all or nothing.
func (s *Store) Save(ctx context.Context, a entity.Aggregate, claims ...entity.Claim) (int64, error) {
	if len(a.Pending()) == 0 && len(claims) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	streamID := a.StreamID()
	before := len(s.streams[streamID])

	if before != a.Version() {
		return 0, fmt.Errorf("memstore - Save %s at version %d, found %d: %w",
			streamID, a.Version(), before, entity.ErrConcurrencyConflict)
	}

	if err := s.checkClaims(claims); err != nil {
		return 0, err
	}

	stored := make([]eventcodec.Stored, 0, len(a.Pending()))

	for _, e := range a.Pending() {
		st, err := s.codec.Encode(e)
		if err != nil {
			return 0, fmt.Errorf("memstore - Save: %w", err)
		}

		stored = append(stored, st)
	}

	s.applyClaims(claims)

	if len(stored) == 0 {
		return 0, nil
	}

	md := eventcodec.Metadata(ctx)

	for i, st := range stored {
		s.position++
		s.streams[streamID] = append(s.streams[streamID], row{
			position: s.position, version: before + i + 1, stored: st, metadata: md, recordedAt: time.Now(),
		})
	}

	if err := s.snapshotIfDue(a, before, before+len(stored)); err != nil {
		return 0, err
	}

	entity.Saved(a)

	return s.position, nil
}

func (s *Store) checkClaims(claims []entity.Claim) error {
	for _, c := range claims {
		owner, held := s.claims[claimKey{scope: c.Scope, value: c.Value}]
		if c.Release || !held || owner == c.Owner {
			continue
		}

		return eventcodec.ClaimTaken(&c)
	}

	return nil
}

func (s *Store) applyClaims(claims []entity.Claim) {
	for _, c := range claims {
		key := claimKey{scope: c.Scope, value: c.Value}

		switch {
		case !c.Release:
			s.claims[key] = c.Owner
		case s.claims[key] == c.Owner:
			delete(s.claims, key)
		}
	}
}

func (s *Store) snapshotIfDue(a entity.Aggregate, before, after int) error {
	sn, ok := a.(entity.Snapshotter)
	if !ok || !eventcodec.SnapshotDue(before, after, s.snapshotEvery) {
		return nil
	}

	payload, err := json.Marshal(sn.Snapshot())
	if err != nil {
		return fmt.Errorf("memstore - Save - snapshot: %w", err)
	}

	s.snapshots[a.StreamID()] = snapshot{version: after, schemaVersion: sn.SnapshotVersion(), payload: payload}

	return nil
}

// Load replays streamID into a, from its snapshot when a can use it.
func (s *Store) Load(_ context.Context, streamID string, a entity.Aggregate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := s.streams[streamID]
	if len(rows) == 0 {
		return fmt.Errorf("memstore - Load %s: %w", streamID, entity.ErrStreamNotFound)
	}

	from := 0

	if snap, ok := s.snapshots[streamID]; ok {
		if sn, isSnapshotter := a.(entity.Snapshotter); isSnapshotter && sn.SnapshotVersion() == snap.schemaVersion {
			if err := sn.RestoreSnapshot(snap.payload); err != nil {
				return fmt.Errorf("memstore - Load %s - RestoreSnapshot: %w", streamID, err)
			}

			from = snap.version
		}
	}

	events := make([]entity.Event, 0, len(rows)-from)

	for i := from; i < len(rows); i++ {
		e, err := s.codec.Decode(rows[i].stored)
		if err != nil {
			return fmt.Errorf("memstore - Load %s: %w", streamID, err)
		}

		events = append(events, e)
	}

	entity.Replay(a, len(rows), events...)

	return nil
}

// ClaimOwner returns who holds the claim.
func (s *Store) ClaimOwner(_ context.Context, scope, value string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	owner, ok := s.claims[claimKey{scope: scope, value: value}]
	if !ok {
		return "", fmt.Errorf("memstore - ClaimOwner %s %q: %w", scope, value, entity.ErrClaimNotFound)
	}

	return owner, nil
}

// Events returns the events stored in streamID, in order: what a command-handler test asserts
// after the command ran. Every stored event was encoded by the same codec, so one that does not
// decode is a bug in the store, and panics.
func (s *Store) Events(streamID string) []entity.Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	events := make([]entity.Event, 0, len(s.streams[streamID]))

	for i := range s.streams[streamID] {
		e, err := s.codec.Decode(s.streams[streamID][i].stored)
		if err != nil {
			panic(fmt.Sprintf("memstore - Events %s: %v", streamID, err))
		}

		events = append(events, e)
	}

	return events
}
