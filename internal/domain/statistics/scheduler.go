package statistics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
)

const (
	// Aggregation scheduler configuration
	DefaultAggregationInterval = 5 * time.Minute
	DefaultAggregationLookback = 1 * time.Hour
)

// AggregationScheduler handles periodic aggregation of statistics
type AggregationScheduler struct {
	interval time.Duration
	lookback time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
	started  bool
	mu       sync.RWMutex
}

// NewAggregationScheduler creates a new aggregation scheduler
func NewAggregationScheduler(interval, lookback time.Duration) *AggregationScheduler {
	if interval <= 0 {
		interval = DefaultAggregationInterval
	}
	if lookback <= 0 {
		lookback = DefaultAggregationLookback
	}

	return &AggregationScheduler{
		interval: interval,
		lookback: lookback,
		stopChan: make(chan struct{}),
	}
}

// Start begins the aggregation scheduler
func (s *AggregationScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("aggregation scheduler already started")
	}

	slog.Info("starting aggregation scheduler",
		"interval", s.interval,
		"lookback", s.lookback)

	s.wg.Add(1)
	go s.run(ctx)

	s.started = true
	slog.Info("aggregation scheduler started successfully")
	return nil
}

// Stop gracefully stops the aggregation scheduler
func (s *AggregationScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	slog.Info("stopping aggregation scheduler")

	close(s.stopChan)
	s.wg.Wait()

	s.started = false
	slog.Info("aggregation scheduler stopped successfully")
	return nil
}

// run starts the scheduler processing loop
func (s *AggregationScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run initial aggregation
	s.runAggregation(ctx)

	for {
		select {
		case <-s.stopChan:
			slog.Info("aggregation scheduler stopping")
			return
		case <-ctx.Done():
			slog.Info("aggregation scheduler context cancelled")
			return
		case <-ticker.C:
			s.runAggregation(ctx)
		}
	}
}

// runAggregation performs aggregation for all active redirects
func (s *AggregationScheduler) runAggregation(ctx context.Context) {
	slog.Debug("starting aggregation run")

	// Get all active redirects
	filter := &redirect.Filter{
		Status: redirect.RedirectStatusActive,
		Limit:  1000, // Process up to 1000 redirects per run
	}

	redirects, err := redirect.FindMany(ctx, filter)
	if err != nil {
		slog.Error("failed to fetch redirects for aggregation", "error", err)
		return
	}

	if len(redirects) == 0 {
		slog.Debug("no active redirects found for aggregation")
		return
	}

	slog.Debug("processing redirects for aggregation", "count", len(redirects))

	// Process each redirect
	for _, r := range redirects {
		s.processRedirectAggregation(ctx, r)
	}

	slog.Debug("aggregation run completed", "processed", len(redirects))
}

// processRedirectAggregation creates aggregation jobs for a specific redirect
func (s *AggregationScheduler) processRedirectAggregation(ctx context.Context, r *redirect.Redirect) {
	now := time.Now()

	// Create aggregation jobs for different periods
	periods := []string{PeriodHourly, PeriodDaily, PeriodWeekly, PeriodMonthly}

	for _, period := range periods {
		// Calculate the timestamp for this period
		timestamp := s.calculatePeriodTimestamp(period, now)

		// Check if we already have aggregated stats for this period
		existingStats, err := FindOneAggregatedStats(ctx, &AggregatedStatsFilter{
			RedirectID: r.ID,
			Period:     period,
			StartTime:  &timestamp,
			EndTime:    &timestamp,
		})

		if err == nil && existingStats != nil {
			// Already aggregated, skip
			continue
		}

		// Queue aggregation job
		if err := QueueAggregationJobAsync(ctx, r.ID, r.UserID, period, timestamp); err != nil {
			slog.Error("failed to queue aggregation job",
				"error", err,
				"redirectId", r.ID,
				"period", period,
				"timestamp", timestamp)
		} else {
			slog.Debug("queued aggregation job",
				"redirectId", r.ID,
				"period", period,
				"timestamp", timestamp)
		}
	}
}

// calculatePeriodTimestamp calculates the appropriate timestamp for a given period
func (s *AggregationScheduler) calculatePeriodTimestamp(period string, now time.Time) time.Time {
	switch period {
	case PeriodHourly:
		// Round down to the hour
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	case PeriodDaily:
		// Round down to the day
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case PeriodWeekly:
		// Round down to the start of the week (Monday)
		weekday := int(now.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start.AddDate(0, 0, -(weekday - 1))
	case PeriodMonthly:
		// Round down to the start of the month
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		// Default to hourly
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	}
}

// Global aggregation scheduler instance
var (
	aggregationScheduler *AggregationScheduler
	aggregationMutex     sync.RWMutex
)

// ConfigureAggregationScheduler configures the global aggregation scheduler
func ConfigureAggregationScheduler(interval, lookback time.Duration) {
	aggregationMutex.Lock()
	defer aggregationMutex.Unlock()

	aggregationScheduler = NewAggregationScheduler(interval, lookback)
}

// StartAggregationScheduler starts the global aggregation scheduler
func StartAggregationScheduler(ctx context.Context) error {
	aggregationMutex.RLock()
	defer aggregationMutex.RUnlock()

	if aggregationScheduler == nil {
		return fmt.Errorf("aggregation scheduler not configured")
	}

	return aggregationScheduler.Start(ctx)
}

// StopAggregationScheduler stops the global aggregation scheduler
func StopAggregationScheduler() error {
	aggregationMutex.RLock()
	defer aggregationMutex.RUnlock()

	if aggregationScheduler == nil {
		return nil
	}

	return aggregationScheduler.Stop()
}
