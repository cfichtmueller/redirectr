package statistics

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	redirectHitCollection     *mongo.Collection
	aggregatedStatsCollection *mongo.Collection
)

func Configure(db *mongo.Database) {
	redirectHitCollection = db.Collection("stats_redirect_hits")
	aggregatedStatsCollection = db.Collection("stats_aggregated_stats")

	// Create indexes for redirect hits collection
	if _, err := redirectHitCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "redirectId", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_redirect_hits_redirect_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_redirect_hits_user_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_redirect_hits_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "ip", Value: 1}},
			Options: options.Index().SetName("idx_redirect_hits_ip"),
		},
		{
			Keys:    bson.D{{Key: "userAgent", Value: 1}},
			Options: options.Index().SetName("idx_redirect_hits_user_agent"),
		},
		{
			Keys:    bson.D{{Key: "requestedUrl", Value: 1}},
			Options: options.Index().SetName("idx_redirect_hits_requested_url"),
		},
	}); err != nil {
		slog.Error("unable to create redirect hits indices", "error", err)
	}

	// Create indexes for aggregated stats collection
	if _, err := aggregatedStatsCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "redirectId", Value: 1}, {Key: "period", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_aggregated_stats_redirect_period_timestamp").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "period", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_aggregated_stats_user_period_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "period", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_aggregated_stats_period_timestamp"),
		},
	}); err != nil {
		slog.Error("unable to create aggregated stats indices", "error", err)
	}
}
