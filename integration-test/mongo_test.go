package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/pkg/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoInsertsReadsAndDropsADocument(t *testing.T) {
	mongoURL := os.Getenv("MONGO_URL")
	require.NotEmpty(t, mongoURL, "MONGO_URL is not set")

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	db, disconnect, err := mongo.New(ctx, mongoURL)
	require.NoError(t, err)

	defer func() { assert.NoError(t, disconnect(context.WithoutCancel(ctx))) }()

	coll := db.Collection("_test_" + uuid.NewString())
	defer func() { assert.NoError(t, coll.Drop(context.WithoutCancel(ctx))) }()

	id := uuid.NewString()
	_, err = coll.InsertOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "value", Value: "round trip"}})
	require.NoError(t, err)

	var got struct {
		Value string `bson:"value"`
	}

	require.NoError(t, coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&got))
	assert.Equal(t, "round trip", got.Value)
}
