package statistics

import (
	"context"
	"fmt"
	"time"

	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/redirectr/internal/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	PeriodHourly  = "hourly"
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
)

var Periods = []string{
	PeriodHourly,
	PeriodDaily,
	PeriodWeekly,
	PeriodMonthly,
}

//
// Commands
//

type RecordHitCommand struct {
	RedirectID   string
	UserID       string
	IP           string
	UserAgent    string
	Referer      string
	RequestedURL string // Path and query parameters (e.g., "/path?param=value")
	Timestamp    time.Time
}

type CreateAggregatedStatsCommand struct {
	RedirectID string
	UserID     string
	Period     string
	Timestamp  time.Time
	HitCount   int64
	UniqueIPs  int64
}

//
// Models
//

// RedirectHit represents a single redirect hit/access
type RedirectHit struct {
	ID           string    `bson:"_id"`
	RedirectID   string    `bson:"redirectId"`
	UserID       string    `bson:"userId"`
	IP           string    `bson:"ip"`
	UserAgent    string    `bson:"userAgent"`
	Referer      string    `bson:"referer"`
	RequestedURL string    `bson:"requestedUrl"` // Path and query parameters (e.g., "/path?param=value")
	Timestamp    time.Time `bson:"timestamp"`
}

// AggregatedStats represents aggregated statistics for a specific time period
type AggregatedStats struct {
	ID         string    `bson:"_id"`
	RedirectID string    `bson:"redirectId"`
	UserID     string    `bson:"userId"`
	Period     string    `bson:"period"`
	Timestamp  time.Time `bson:"timestamp"`
	HitCount   int64     `bson:"hitCount"`
	UniqueIPs  int64     `bson:"uniqueIps"`
	CreatedAt  time.Time `bson:"createdAt"`
	UpdatedAt  time.Time `bson:"updatedAt"`
}

//
// Filter
//

type RedirectHitFilter struct {
	ID           string
	IDs          []string
	RedirectID   string
	UserID       string
	IP           string
	UserAgent    string
	RequestedURL string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

func (f *RedirectHitFilter) bson() bson.D {
	d := util.NewFilter().
		AppendIf(f.ID != "", "_id", f.ID).
		AppendIf(f.IDs != nil, "_id", bson.M{"$in": f.IDs}).
		AppendIf(f.RedirectID != "", "redirectId", f.RedirectID).
		AppendIf(f.UserID != "", "userId", f.UserID).
		AppendIf(f.IP != "", "ip", f.IP).
		AppendIf(f.UserAgent != "", "userAgent", f.UserAgent).
		AppendIf(f.RequestedURL != "", "requestedUrl", f.RequestedURL).
		Bson()

	// Add time range filter
	if f.StartTime != nil || f.EndTime != nil {
		timeFilter := bson.M{}
		if f.StartTime != nil {
			timeFilter["$gte"] = *f.StartTime
		}
		if f.EndTime != nil {
			timeFilter["$lte"] = *f.EndTime
		}
		d = append(d, bson.E{Key: "timestamp", Value: timeFilter})
	}

	return d
}

type AggregatedStatsFilter struct {
	ID         string
	IDs        []string
	RedirectID string
	UserID     string
	Period     string
	Periods    []string
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}

func (f *AggregatedStatsFilter) bson() bson.D {
	d := util.NewFilter().
		AppendIf(f.ID != "", "_id", f.ID).
		AppendIf(f.IDs != nil, "_id", bson.M{"$in": f.IDs}).
		AppendIf(f.RedirectID != "", "redirectId", f.RedirectID).
		AppendIf(f.UserID != "", "userId", f.UserID).
		AppendIf(f.Period != "", "period", f.Period).
		AppendIf(f.Periods != nil, "period", bson.M{"$in": f.Periods}).
		Bson()

	// Add time range filter
	if f.StartTime != nil || f.EndTime != nil {
		timeFilter := bson.M{}
		if f.StartTime != nil {
			timeFilter["$gte"] = *f.StartTime
		}
		if f.EndTime != nil {
			timeFilter["$lte"] = *f.EndTime
		}
		d = append(d, bson.E{Key: "timestamp", Value: timeFilter})
	}

	return d
}

//
// Methods
//

// RecordHit records a redirect hit
func RecordHit(ctx context.Context, cmd RecordHitCommand) (*RedirectHit, error) {
	hit := &RedirectHit{
		ID:           util.RandomId(),
		RedirectID:   cmd.RedirectID,
		UserID:       cmd.UserID,
		IP:           cmd.IP,
		UserAgent:    cmd.UserAgent,
		Referer:      cmd.Referer,
		RequestedURL: cmd.RequestedURL,
		Timestamp:    cmd.Timestamp,
	}

	if _, err := redirectHitCollection.InsertOne(ctx, hit); err != nil {
		return nil, fmt.Errorf("unable to record redirect hit: %w", err)
	}

	return hit, nil
}

// CountRedirectHits returns the count of redirect hits matching the filter
func CountRedirectHits(ctx context.Context, filter *RedirectHitFilter) (int64, error) {
	count, err := redirectHitCollection.CountDocuments(ctx, filter.bson())
	if err != nil {
		return 0, fmt.Errorf("unable to count redirect hits: %w", err)
	}
	return count, nil
}

// CreateAggregatedStats creates or updates aggregated statistics
func CreateAggregatedStats(ctx context.Context, cmd CreateAggregatedStatsCommand) (*AggregatedStats, error) {
	now := time.Now()

	// Create unique identifier for this aggregation period
	periodKey := fmt.Sprintf("%s_%s_%d", cmd.RedirectID, cmd.Period, cmd.Timestamp.Unix())

	stats := &AggregatedStats{
		ID:         periodKey,
		RedirectID: cmd.RedirectID,
		UserID:     cmd.UserID,
		Period:     cmd.Period,
		Timestamp:  cmd.Timestamp,
		HitCount:   cmd.HitCount,
		UniqueIPs:  cmd.UniqueIPs,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Use ReplaceOne with upsert to create or update
	filter := bson.M{
		"redirectId": cmd.RedirectID,
		"period":     cmd.Period,
		"timestamp":  cmd.Timestamp,
	}

	opts := options.Replace().SetUpsert(true)
	if _, err := aggregatedStatsCollection.ReplaceOne(ctx, filter, stats, opts); err != nil {
		return nil, fmt.Errorf("unable to create aggregated stats: %w", err)
	}

	return stats, nil
}

// FindOneAggregatedStats returns a single aggregated stats matching the filter
func FindOneAggregatedStats(ctx context.Context, filter *AggregatedStatsFilter) (*AggregatedStats, error) {
	return findOneAggregatedStats(ctx, filter.bson())
}

// GetUniqueIPsForPeriod returns unique IP addresses for a specific redirect and time period
func GetUniqueIPsForPeriod(ctx context.Context, redirectID string, startTime, endTime time.Time) ([]string, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"redirectId": redirectID,
				"timestamp": bson.M{
					"$gte": startTime,
					"$lte": endTime,
				},
			},
		},
		{
			"$group": bson.M{
				"_id": "$ip",
			},
		},
		{
			"$project": bson.M{
				"ip":  "$_id",
				"_id": 0,
			},
		},
	}

	cursor, err := redirectHitCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("unable to get unique IPs: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		IP string `bson:"ip"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("unable to decode unique IPs: %w", err)
	}

	ips := make([]string, len(results))
	for i, result := range results {
		ips[i] = result.IP
	}

	return ips, nil
}

// Helper functions

func findOneAggregatedStats(ctx context.Context, filter interface{}) (*AggregatedStats, error) {
	var result AggregatedStats
	if err := aggregatedStatsCollection.FindOne(ctx, filter).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ec.NoSuchAggregatedStats
		}
		return nil, fmt.Errorf("unable to find aggregated stats: %w", err)
	}
	return &result, nil
}
