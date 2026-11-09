package task

import (
	"context"
	"errors"
	"fmt"

	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const _collection = "tasks_attributes"

type document struct {
	ID                   string `bson:"_id"`
	mongotask.Attributes `bson:",inline"`
}

type store struct {
	coll *mgo.Collection
}

var _ repo.TaskAttributesStore = (*store)(nil)

func NewAttributesStore(db *mgo.Database) repo.TaskAttributesStore {
	return &store{coll: db.Collection(_collection)}
}

func (s *store) Get(ctx context.Context, taskID string) (mongotask.Attributes, error) {
	var doc document

	err := s.coll.FindOne(ctx, bson.D{{Key: "_id", Value: taskID}}).Decode(&doc)
	if errors.Is(err, mgo.ErrNoDocuments) {
		return mongotask.Attributes{}, nil
	}

	if err != nil {
		return mongotask.Attributes{}, fmt.Errorf("TaskAttributesStore - Get: %w", err)
	}

	return doc.Attributes, nil
}

func (s *store) GetMany(ctx context.Context, taskIDs []string) (map[string]mongotask.Attributes, error) {
	out := make(map[string]mongotask.Attributes, len(taskIDs))
	if len(taskIDs) == 0 {
		return out, nil
	}

	cur, err := s.coll.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: taskIDs}}}})
	if err != nil {
		return nil, fmt.Errorf("TaskAttributesStore - GetMany - Find: %w", err)
	}

	var docs []document
	if err = cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("TaskAttributesStore - GetMany - All: %w", err)
	}

	for _, d := range docs {
		out[d.ID] = d.Attributes
	}

	return out, nil
}

func (s *store) Patch(ctx context.Context, taskID string, p mongotask.AttributesPatch) error {
	set, unset := p.Changes()

	update := bson.D{}
	if len(set) > 0 {
		update = append(update, bson.E{Key: "$set", Value: set})
	}

	if len(unset) > 0 {
		fields := bson.D{}
		for _, f := range unset {
			fields = append(fields, bson.E{Key: f, Value: ""})
		}

		update = append(update, bson.E{Key: "$unset", Value: fields})
	}

	if len(update) == 0 {
		return nil
	}

	_, err := s.coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: taskID}}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("TaskAttributesStore - Patch: %w", err)
	}

	return nil
}

func (s *store) Delete(ctx context.Context, taskID string) error {
	if _, err := s.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: taskID}}); err != nil {
		return fmt.Errorf("TaskAttributesStore - Delete: %w", err)
	}

	return nil
}
