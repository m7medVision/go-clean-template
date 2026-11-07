package eventstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

const (
	// _lockKey is the advisory lock every append holds until it commits.
	_lockKey int64 = 0x6576656e7473 // "events"

	_uniqueViolation = "23505"
)

// Store is the Postgres event store.
type Store struct {
	pg            *postgres.Postgres
	codec         *eventcodec.Codec
	snapshotEvery int
}

// New returns the event store, encoding events with codec. It snapshots a stream each time it
// crosses a multiple of snapshotEvery events; below 1, never.
func New(pg *postgres.Postgres, codec *eventcodec.Codec, snapshotEvery int) repo.EventStore {
	return &Store{pg: pg, codec: codec, snapshotEvery: snapshotEvery}
}

// Save appends a's pending events and applies claims in one transaction, joining the context's.
func (s *Store) Save(ctx context.Context, a entity.Aggregate, claims ...entity.Claim) (int64, error) {
	if len(a.Pending()) == 0 && len(claims) == 0 {
		return 0, nil
	}

	var last int64

	err := s.pg.InTx(ctx, func(ctx context.Context) error {
		q := queries.New(s.pg.Executor(ctx))

		if err := q.LockEventStore(ctx, _lockKey); err != nil {
			return fmt.Errorf("q.LockEventStore: %w", err)
		}

		if err := checkVersion(ctx, q, a); err != nil {
			return err
		}

		if err := applyClaims(ctx, q, claims); err != nil {
			return err
		}

		var err error
		if last, err = s.append(ctx, q, a); err != nil {
			return err
		}

		return s.snapshotIfDue(ctx, q, a)
	})
	if err != nil {
		return 0, fmt.Errorf("EventStore - Save %s: %w", a.StreamID(), err)
	}

	entity.Saved(a)

	return last, nil
}

func checkVersion(ctx context.Context, q *queries.Queries, a entity.Aggregate) error {
	version, err := q.StreamVersion(ctx, a.StreamID())
	if err != nil {
		return fmt.Errorf("q.StreamVersion: %w", err)
	}

	if int(version) != a.Version() {
		return fmt.Errorf("loaded at version %d, stored at %d: %w", a.Version(), version, entity.ErrConcurrencyConflict)
	}

	return nil
}

func applyClaims(ctx context.Context, q *queries.Queries, claims []entity.Claim) error {
	for _, c := range claims {
		if c.Release {
			err := q.ReleaseClaim(ctx, queries.ReleaseClaimParams{Scope: c.Scope, Value: c.Value, OwnerID: c.Owner})
			if err != nil {
				return fmt.Errorf("q.ReleaseClaim: %w", err)
			}

			continue
		}

		claimed, err := q.InsertClaim(ctx, queries.InsertClaimParams{Scope: c.Scope, Value: c.Value, OwnerID: c.Owner})
		if err != nil {
			return fmt.Errorf("q.InsertClaim: %w", err)
		}

		if claimed == 0 {
			return eventcodec.ClaimTaken(&c)
		}
	}

	return nil
}

// append inserts the pending events after the loaded version and returns the last position.
func (s *Store) append(ctx context.Context, q *queries.Queries, a entity.Aggregate) (int64, error) {
	if len(a.Pending()) == 0 {
		return 0, nil
	}

	metadata, err := json.Marshal(eventcodec.Metadata(ctx))
	if err != nil {
		return 0, fmt.Errorf("json.Marshal metadata: %w", err)
	}

	var last int64

	for i, e := range a.Pending() {
		stored, err := s.codec.Encode(e)
		if err != nil {
			return 0, err
		}

		last, err = q.AppendEvent(ctx, queries.AppendEventParams{
			StreamID:      a.StreamID(),
			StreamVersion: toInt32(a.Version() + i + 1),
			Type:          stored.Type,
			SchemaVersion: toInt32(stored.SchemaVersion),
			Payload:       stored.Payload,
			Metadata:      metadata,
		})
		if err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == _uniqueViolation {
				return 0, fmt.Errorf("q.AppendEvent: %w", entity.ErrConcurrencyConflict)
			}

			return 0, fmt.Errorf("q.AppendEvent: %w", err)
		}
	}

	if err = q.NotifyEvents(ctx); err != nil {
		return 0, fmt.Errorf("q.NotifyEvents: %w", err)
	}

	return last, nil
}

func (s *Store) snapshotIfDue(ctx context.Context, q *queries.Queries, a entity.Aggregate) error {
	sn, ok := a.(entity.Snapshotter)
	after := a.Version() + len(a.Pending())

	if !ok || !eventcodec.SnapshotDue(a.Version(), after, s.snapshotEvery) {
		return nil
	}

	payload, err := json.Marshal(sn.Snapshot())
	if err != nil {
		return fmt.Errorf("json.Marshal snapshot: %w", err)
	}

	err = q.PutSnapshot(ctx, queries.PutSnapshotParams{
		StreamID:      a.StreamID(),
		StreamVersion: toInt32(after),
		SchemaVersion: toInt32(sn.SnapshotVersion()),
		Payload:       payload,
	})
	if err != nil {
		return fmt.Errorf("q.PutSnapshot: %w", err)
	}

	return nil
}

// Load replays streamID into a: its snapshot when a can use it, then the events after it.
func (s *Store) Load(ctx context.Context, streamID string, a entity.Aggregate) error {
	q := queries.New(s.pg.Executor(ctx))

	from, err := restoreSnapshot(ctx, q, streamID, a)
	if err != nil {
		return fmt.Errorf("EventStore - Load %s: %w", streamID, err)
	}

	rows, err := q.ReadStream(ctx, queries.ReadStreamParams{StreamID: streamID, AfterVersion: toInt32(from)})
	if err != nil {
		return fmt.Errorf("EventStore - Load %s - q.ReadStream: %w", streamID, err)
	}

	if from == 0 && len(rows) == 0 {
		return fmt.Errorf("EventStore - Load %s: %w", streamID, entity.ErrStreamNotFound)
	}

	events := make([]entity.Event, 0, len(rows))
	version := from

	for i := range rows {
		e, err := s.codec.Decode(eventcodec.Stored{Type: rows[i].Type, SchemaVersion: int(rows[i].SchemaVersion), Payload: rows[i].Payload})
		if err != nil {
			return fmt.Errorf("EventStore - Load %s: %w", streamID, err)
		}

		events = append(events, e)
		version = int(rows[i].StreamVersion)
	}

	entity.Replay(a, version, events...)

	return nil
}

// restoreSnapshot restores a from streamID's snapshot when a is a Snapshotter of the snapshot's
// version, and returns the stream version it restored to (0 for none).
func restoreSnapshot(ctx context.Context, q *queries.Queries, streamID string, a entity.Aggregate) (int, error) {
	sn, ok := a.(entity.Snapshotter)
	if !ok {
		return 0, nil
	}

	snap, err := q.GetSnapshot(ctx, streamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("q.GetSnapshot: %w", err)
	}

	if int(snap.SchemaVersion) != sn.SnapshotVersion() {
		return 0, nil
	}

	if err = sn.RestoreSnapshot(snap.Payload); err != nil {
		return 0, fmt.Errorf("RestoreSnapshot: %w", err)
	}

	return int(snap.StreamVersion), nil
}

// ClaimOwner returns who holds the claim.
func (s *Store) ClaimOwner(ctx context.Context, scope, value string) (string, error) {
	owner, err := queries.New(s.pg.Executor(ctx)).ClaimOwner(ctx, queries.ClaimOwnerParams{Scope: scope, Value: value})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("EventStore - ClaimOwner %s %q: %w", scope, value, entity.ErrClaimNotFound)
	}

	if err != nil {
		return "", fmt.Errorf("EventStore - ClaimOwner - q.ClaimOwner: %w", err)
	}

	return owner, nil
}

// toInt32 clamps v to the int32 range the stream version and schema version columns hold.
func toInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(v) //nolint:gosec // clamped above; versions are never negative
}
