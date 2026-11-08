package mongo

import (
	"context"
	"errors"
	"fmt"

	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

var ErrNoDatabase = errors.New("mongo: the URL names no database")

func New(ctx context.Context, url string) (*mgo.Database, func(context.Context) error, error) {
	cs, err := connstring.ParseAndValidate(url)
	if err != nil {
		return nil, nil, fmt.Errorf("mongo - New - connstring.ParseAndValidate: %w", err)
	}

	if cs.Database == "" {
		return nil, nil, ErrNoDatabase
	}

	client, err := mgo.Connect(options.Client().ApplyURI(url).SetMonitor(newMonitor()))
	if err != nil {
		return nil, nil, fmt.Errorf("mongo - New - mgo.Connect: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("mongo - New - ping: %w", errors.Join(err, client.Disconnect(context.WithoutCancel(ctx))))
	}

	return client.Database(cs.Database), client.Disconnect, nil
}
