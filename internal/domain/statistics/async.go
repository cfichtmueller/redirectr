package statistics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// Channel buffer sizes
	DefaultHitBufferSize = 1000
	DefaultAggBufferSize = 100

	// Worker configuration
	DefaultWorkerCount    = 3
	DefaultBatchSize      = 100
	DefaultProcessTimeout = 30 * time.Second
)

// AsyncStatsService handles asynchronous statistics collection and processing
type AsyncStatsService struct {
	workerCount    int
	batchSize      int
	processTimeout time.Duration

	// Channels for different types of jobs
	hitChan chan RedirectHitJob
	aggChan chan AggregationJob

	workers  []*statsWorker
	stopChan chan struct{}
	wg       sync.WaitGroup
	started  bool
	mu       sync.RWMutex
}

// statsWorker processes statistics from channels
type statsWorker struct {
	id       int
	service  *AsyncStatsService
	stopChan chan struct{}
}

// RedirectHitJob represents a job to record a redirect hit
type RedirectHitJob struct {
	RedirectID   string    `json:"redirectId"`
	UserID       string    `json:"userId"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"userAgent"`
	Referer      string    `json:"referer"`
	RequestedURL string    `json:"requestedUrl"` // Path and query parameters (e.g., "/path?param=value")
	Timestamp    time.Time `json:"timestamp"`
}

// AggregationJob represents a job to create aggregated statistics
type AggregationJob struct {
	RedirectID string    `json:"redirectId"`
	UserID     string    `json:"userId"`
	Period     string    `json:"period"`
	Timestamp  time.Time `json:"timestamp"`
}

// NewAsyncStatsService creates a new async statistics service
func NewAsyncStatsService(workerCount, batchSize int, processTimeout time.Duration, hitBufferSize, aggBufferSize int) *AsyncStatsService {
	if workerCount <= 0 {
		workerCount = DefaultWorkerCount
	}
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	if processTimeout <= 0 {
		processTimeout = DefaultProcessTimeout
	}
	if hitBufferSize <= 0 {
		hitBufferSize = DefaultHitBufferSize
	}
	if aggBufferSize <= 0 {
		aggBufferSize = DefaultAggBufferSize
	}

	return &AsyncStatsService{
		workerCount:    workerCount,
		batchSize:      batchSize,
		processTimeout: processTimeout,
		hitChan:        make(chan RedirectHitJob, hitBufferSize),
		aggChan:        make(chan AggregationJob, aggBufferSize),
		stopChan:       make(chan struct{}),
	}
}

// Start begins processing statistics jobs
func (s *AsyncStatsService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("async stats service already started")
	}

	slog.Info("starting async statistics service",
		"workerCount", s.workerCount,
		"batchSize", s.batchSize,
		"processTimeout", s.processTimeout,
		"hitBufferSize", cap(s.hitChan),
		"aggBufferSize", cap(s.aggChan))

	// Start workers
	s.workers = make([]*statsWorker, s.workerCount)
	for i := 0; i < s.workerCount; i++ {
		worker := &statsWorker{
			id:       i + 1,
			service:  s,
			stopChan: make(chan struct{}),
		}
		s.workers[i] = worker
		s.wg.Add(1)
		go worker.run(ctx)
	}

	s.started = true
	slog.Info("async statistics service started successfully")
	return nil
}

// Stop gracefully stops the async statistics service
func (s *AsyncStatsService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	slog.Info("stopping async statistics service")

	// Signal all workers to stop
	for _, worker := range s.workers {
		close(worker.stopChan)
	}

	// Close main stop channel
	close(s.stopChan)

	// Wait for all workers to finish
	s.wg.Wait()

	s.started = false
	slog.Info("async statistics service stopped successfully")
	return nil
}

// QueueRedirectHit queues a redirect hit for async processing
func (s *AsyncStatsService) QueueRedirectHit(ctx context.Context, cmd RecordHitCommand) error {
	job := RedirectHitJob{
		RedirectID:   cmd.RedirectID,
		UserID:       cmd.UserID,
		IP:           cmd.IP,
		UserAgent:    cmd.UserAgent,
		Referer:      cmd.Referer,
		RequestedURL: cmd.RequestedURL,
		Timestamp:    cmd.Timestamp,
	}

	select {
	case s.hitChan <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel is full, drop the job to avoid blocking
		slog.Warn("redirect hit channel full, dropping job",
			"redirectId", job.RedirectID,
			"channelSize", cap(s.hitChan))
		return fmt.Errorf("redirect hit channel full")
	}
}

// QueueAggregationJob queues an aggregation job for async processing
func (s *AsyncStatsService) QueueAggregationJob(ctx context.Context, redirectID, userID, period string, timestamp time.Time) error {
	job := AggregationJob{
		RedirectID: redirectID,
		UserID:     userID,
		Period:     period,
		Timestamp:  timestamp,
	}

	select {
	case s.aggChan <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel is full, drop the job to avoid blocking
		slog.Warn("aggregation job channel full, dropping job",
			"redirectId", redirectID,
			"period", period,
			"channelSize", cap(s.aggChan))
		return fmt.Errorf("aggregation job channel full")
	}
}

// run starts the worker processing loop
func (w *statsWorker) run(ctx context.Context) {
	defer w.service.wg.Done()

	slog.Info("stats worker started", "workerId", w.id)

	for {
		select {
		case <-w.stopChan:
			slog.Info("stats worker stopping", "workerId", w.id)
			return
		case <-ctx.Done():
			slog.Info("stats worker context cancelled", "workerId", w.id)
			return
		case job := <-w.service.hitChan:
			w.processRedirectHitJob(ctx, job)
		case job := <-w.service.aggChan:
			w.processAggregationJob(ctx, job)
		}
	}
}

// processRedirectHitJob processes a single redirect hit job
func (w *statsWorker) processRedirectHitJob(ctx context.Context, job RedirectHitJob) {
	// Process the job with timeout
	processCtx, cancel := context.WithTimeout(ctx, w.service.processTimeout)
	defer cancel()

	// Record the hit
	cmd := RecordHitCommand{
		RedirectID:   job.RedirectID,
		UserID:       job.UserID,
		IP:           job.IP,
		UserAgent:    job.UserAgent,
		Referer:      job.Referer,
		RequestedURL: job.RequestedURL,
		Timestamp:    job.Timestamp,
	}

	hit, err := RecordHit(processCtx, cmd)
	if err != nil {
		slog.Error("failed to record redirect hit",
			"error", err,
			"redirectId", job.RedirectID,
			"workerId", w.id)
		return
	}

	slog.Debug("redirect hit recorded",
		"hitId", hit.ID,
		"redirectId", hit.RedirectID,
		"workerId", w.id)
}

// processAggregationJob processes a single aggregation job
func (w *statsWorker) processAggregationJob(ctx context.Context, job AggregationJob) {
	// Process the job with timeout
	processCtx, cancel := context.WithTimeout(ctx, w.service.processTimeout)
	defer cancel()

	// Calculate aggregated statistics for the period
	startTime, endTime := calculatePeriodBounds(job.Period, job.Timestamp)

	// Count hits for this period
	hitCount, err := CountRedirectHits(processCtx, &RedirectHitFilter{
		RedirectID: job.RedirectID,
		StartTime:  &startTime,
		EndTime:    &endTime,
	})
	if err != nil {
		slog.Error("failed to count redirect hits",
			"error", err,
			"redirectId", job.RedirectID,
			"period", job.Period,
			"workerId", w.id)
		return
	}

	// Get unique IPs for this period
	uniqueIPs, err := GetUniqueIPsForPeriod(processCtx, job.RedirectID, startTime, endTime)
	if err != nil {
		slog.Error("failed to get unique IPs",
			"error", err,
			"redirectId", job.RedirectID,
			"period", job.Period,
			"workerId", w.id)
		return
	}

	// Create aggregated stats
	cmd := CreateAggregatedStatsCommand{
		RedirectID: job.RedirectID,
		UserID:     job.UserID,
		Period:     job.Period,
		Timestamp:  job.Timestamp,
		HitCount:   hitCount,
		UniqueIPs:  int64(len(uniqueIPs)),
	}

	stats, err := CreateAggregatedStats(processCtx, cmd)
	if err != nil {
		slog.Error("failed to create aggregated stats",
			"error", err,
			"redirectId", job.RedirectID,
			"period", job.Period,
			"workerId", w.id)
		return
	}

	slog.Debug("aggregated stats created",
		"statsId", stats.ID,
		"redirectId", stats.RedirectID,
		"period", stats.Period,
		"hitCount", stats.HitCount,
		"uniqueIPs", stats.UniqueIPs,
		"workerId", w.id)
}

// calculatePeriodBounds calculates the start and end time for a given period
func calculatePeriodBounds(period string, timestamp time.Time) (time.Time, time.Time) {
	switch period {
	case PeriodHourly:
		start := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), timestamp.Hour(), 0, 0, 0, timestamp.Location())
		return start, start.Add(time.Hour)
	case PeriodDaily:
		start := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
		return start, start.Add(24 * time.Hour)
	case PeriodWeekly:
		// Start of week (Monday)
		weekday := int(timestamp.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		start := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
		start = start.AddDate(0, 0, -(weekday - 1))
		return start, start.Add(7 * 24 * time.Hour)
	case PeriodMonthly:
		start := time.Date(timestamp.Year(), timestamp.Month(), 1, 0, 0, 0, 0, timestamp.Location())
		return start, start.AddDate(0, 1, 0)
	default:
		// Default to hourly
		start := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), timestamp.Hour(), 0, 0, 0, timestamp.Location())
		return start, start.Add(time.Hour)
	}
}

// Global async stats service instance
var (
	asyncStatsService *AsyncStatsService
	asyncStatsMutex   sync.RWMutex
)

// ConfigureAsyncStats configures the global async statistics service
func ConfigureAsyncStats(workerCount, batchSize int, processTimeout time.Duration, hitBufferSize, aggBufferSize int) {
	asyncStatsMutex.Lock()
	defer asyncStatsMutex.Unlock()

	asyncStatsService = NewAsyncStatsService(workerCount, batchSize, processTimeout, hitBufferSize, aggBufferSize)
}

// StartAsyncStats starts the global async statistics service
func StartAsyncStats(ctx context.Context) error {
	asyncStatsMutex.RLock()
	defer asyncStatsMutex.RUnlock()

	if asyncStatsService == nil {
		return fmt.Errorf("async stats service not configured")
	}

	return asyncStatsService.Start(ctx)
}

// StopAsyncStats stops the global async statistics service
func StopAsyncStats() error {
	asyncStatsMutex.RLock()
	defer asyncStatsMutex.RUnlock()

	if asyncStatsService == nil {
		return nil
	}

	return asyncStatsService.Stop()
}

// QueueRedirectHitAsync queues a redirect hit for async processing using the global service
func QueueRedirectHitAsync(ctx context.Context, cmd RecordHitCommand) error {
	asyncStatsMutex.RLock()
	defer asyncStatsMutex.RUnlock()

	if asyncStatsService == nil {
		return fmt.Errorf("async stats service not configured")
	}

	return asyncStatsService.QueueRedirectHit(ctx, cmd)
}

// QueueAggregationJobAsync queues an aggregation job for async processing using the global service
func QueueAggregationJobAsync(ctx context.Context, redirectID, userID, period string, timestamp time.Time) error {
	asyncStatsMutex.RLock()
	defer asyncStatsMutex.RUnlock()

	if asyncStatsService == nil {
		return fmt.Errorf("async stats service not configured")
	}

	return asyncStatsService.QueueAggregationJob(ctx, redirectID, userID, period, timestamp)
}
