package config

import (
	"context"
	"fmt"
	"time"

	"github.com/cfichtmueller/redirectr/internal/infra/health"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func CreateMongoClient() (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().
		ApplyURI(MongoUri).
		SetTimeout(30 * time.Second).
		SetReadPreference(readpref.Nearest())
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to mongodb: %w", err)
	}
	return client, client.Database(MongoDatabase), nil
}

type mongoHealthIndicator struct {
	db *mongo.Database
}

func CreateMongoHealthIndicator(db *mongo.Database) health.Indicator {
	return &mongoHealthIndicator{
		db: db,
	}
}

func (i *mongoHealthIndicator) GetHealth(ctx context.Context) (*health.Status, error) {
	_, err := i.db.ListCollections(ctx, bson.D{}, nil)
	if err != nil {
		return health.StatusDown("mongodb").AddDetail("error", err), nil
	}
	return health.StatusUp("mongodb"), nil
}
