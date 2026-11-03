package eventstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Reader reads the whole store in position order, for the projection runner.
type Reader struct {
	pg             *postgres.Postgres
	codec          *Codec
	includeScratch bool
}

// NewReader returns a reader for events (entity.EventTypes() for the domain's own), upcasting
// their old versions with upcasts. It skips scratch streams, whose IDs start with an underscore.
func NewReader(pg *postgres.Postgres, events []entity.Event, upcasts ...Upcast) *Reader {
	return &Reader{pg: pg, codec: NewCodec(events, upcasts...)}
}

// IncludingScratch returns a reader that also reads scratch streams: the store's own tests use it,
// the worker never does.
func (r *Reader) IncludingScratch() *Reader {
	c := *r
	c.includeScratch = true

	return &c
}

// ReadAll returns at most limit events after position after, decoded to their current versions.
func (r *Reader) ReadAll(ctx context.Context, after int64, limit int) ([]entity.RecordedEvent, error) {
	rows, err := queries.New(r.pg.Executor(ctx)).ReadAll(ctx, queries.ReadAllParams{
		AfterPosition:  after,
		IncludeScratch: r.includeScratch,
		MaxEvents:      toInt32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("Reader - ReadAll - q.ReadAll: %w", err)
	}

	events := make([]entity.RecordedEvent, 0, len(rows))

	for i := range rows {
		e, err := r.codec.Decode(Stored{Type: rows[i].Type, SchemaVersion: int(rows[i].SchemaVersion), Payload: rows[i].Payload})
		if err != nil {
			return nil, fmt.Errorf("Reader - ReadAll at %d: %w", rows[i].Position, err)
		}

		var md entity.Metadata
		if err = json.Unmarshal(rows[i].Metadata, &md); err != nil {
			return nil, fmt.Errorf("Reader - ReadAll at %d - metadata: %w", rows[i].Position, err)
		}

		events = append(events, entity.RecordedEvent{
			Position:      rows[i].Position,
			StreamID:      rows[i].StreamID,
			StreamVersion: int(rows[i].StreamVersion),
			Event:         e,
			Metadata:      md,
			RecordedAt:    rows[i].RecordedAt,
		})
	}

	return events, nil
}
