package redirect

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	redirectCollection *mongo.Collection
	lookupService      *LookupService
)

func Configure(db *mongo.Database) {
	// Initialize redirect lookup service with caching
	// Cache capacity: 1000 entries (configurable)
	// Cache TTL: 5 minutes (configurable)
	cacheCapacity := 1000
	cacheTTL := 5 * time.Minute

	redirectCollection = db.Collection("redirects")

	// Create indexes for performance
	if _, err := redirectCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "sourceDomain", Value: 1}},
			Options: options.Index().SetName("idx_redirect_source_domain").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_redirect_user_created"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_redirect_status"),
		},
		{
			Keys:    bson.D{{Key: "sourceDomain", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_redirect_source_status"),
		},
	}); err != nil {
		slog.Error("unable to create redirect indices", "error", err)
	}

	lookupService = NewRedirectLookupService(db, cacheCapacity, cacheTTL)
}
