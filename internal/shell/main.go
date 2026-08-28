package shell

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/cfichtmueller/redirectr/internal/config"
	"github.com/cfichtmueller/redirectr/internal/domain/iam"
	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/domain/statistics"
	"github.com/cfichtmueller/redirectr/internal/infra/audit"
	"github.com/cfichtmueller/redirectr/internal/infra/health"
	"github.com/getsentry/sentry-go"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Shell struct {
	Db             *mongo.Database
	dbCleanup      func()
	HealthEndpoint *health.Endpoint
}

func Configure() (*Shell, error) {
	ctx := context.Background()

	config.Load()

	if config.SentryDsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              config.SentryDsn,
			Environment:      config.Environment,
			Release:          config.Release,
			ServerName:       "redirectr",
			EnableTracing:    true,
			TracesSampleRate: 1.0,
			SendDefaultPII:   true,
		}); err != nil {
			return nil, fmt.Errorf("sentry initialization failed: %w", err)
		}
	}

	db, dbCleanup, err := configureDb()
	if err != nil {
		return nil, err
	}

	healthEndpoint := health.NewEndpoint()
	healthEndpoint.AddIndicator(config.CreateMongoHealthIndicator(db))

	audit.Configure(db)

	iam.Configure(db)
	redirect.Configure(db)
	statistics.Configure(db)

	// Configure async statistics service with channel buffers
	statistics.ConfigureAsyncStats(3, 100, 30*time.Second, 1000, 100)

	// Configure aggregation scheduler
	statistics.ConfigureAggregationScheduler(5*time.Minute, 1*time.Hour)

	// Start async statistics service
	if err := statistics.StartAsyncStats(ctx); err != nil {
		return nil, fmt.Errorf("failed to start async statistics service: %w", err)
	}

	// Start aggregation scheduler
	if err := statistics.StartAggregationScheduler(ctx); err != nil {
		return nil, fmt.Errorf("failed to start aggregation scheduler: %w", err)
	}

	// Warm the cache in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := redirect.WarmCache(ctx, 100); err != nil {
			slog.Error("failed to warm cache", "error", err)
		}
	}()

	return &Shell{
		Db:             db,
		dbCleanup:      dbCleanup,
		HealthEndpoint: healthEndpoint,
	}, nil
}

func (s *Shell) Teardown() {
	// Stop aggregation scheduler
	if err := statistics.StopAggregationScheduler(); err != nil {
		slog.Error("failed to stop aggregation scheduler", "error", err)
	}

	// Stop async statistics service
	if err := statistics.StopAsyncStats(); err != nil {
		slog.Error("failed to stop async statistics service", "error", err)
	}

	// Flush any buffered Sentry events before exiting (no-op if Sentry is disabled)
	sentry.Flush(2 * time.Second)

	s.dbCleanup()
}

func configureDb() (*mongo.Database, func(), error) {
	mongoClient, db, err := config.CreateMongoClient()
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "unable to disconnect from mongo database: %v\n", err)
			os.Exit(1)
		}
	}

	return db, cleanup, nil
}
