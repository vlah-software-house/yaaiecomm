package vat

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Scheduler runs the VAT rate sync on startup and then daily at midnight UTC.
// It uses a simple approach: calculate the duration until the next midnight UTC,
// sleep until then, perform the sync, and repeat every 24 hours.
type Scheduler struct {
	syncer *RateSyncer
	logger *slog.Logger

	stopCh chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

// NewScheduler creates a new VAT sync scheduler.
func NewScheduler(syncer *RateSyncer, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		syncer: syncer,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduler. It runs an initial sync immediately, then
// schedules syncs at midnight UTC every day. The context is used for the
// initial sync; the daily syncs use a background context that respects
// the stop signal.
//
// Start is non-blocking: it runs the daily loop in a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	// Run initial sync immediately.
	s.logger.Info("running initial VAT rate sync on startup")
	result := s.syncer.Sync(ctx)
	if result.Error != nil {
		s.logger.Error("initial VAT rate sync failed",
			"error", result.Error,
		)
	} else {
		s.logger.Info("initial VAT rate sync completed",
			"source", result.Source,
			"rates_loaded", result.RatesLoaded,
			"rates_changed", result.RatesChanged,
		)
	}

	// Start daily sync loop in a goroutine.
	s.wg.Add(1)
	go s.loop()
}

// Stop signals the scheduler to stop and waits for it to finish.
// It is safe to call Stop multiple times.
func (s *Scheduler) Stop() {
	s.once.Do(func() {
		s.logger.Info("stopping VAT rate sync scheduler")
		close(s.stopCh)
	})
	s.wg.Wait()
}

// loop runs the daily sync cycle. It calculates the duration until the next
// midnight UTC, waits, syncs, and then repeats with a 24-hour ticker.
func (s *Scheduler) loop() {
	defer s.wg.Done()

	// Calculate duration until next midnight UTC.
	untilMidnight := durationUntilNextMidnightUTC()
	s.logger.Info("VAT sync scheduler: next sync scheduled",
		"in", untilMidnight.Round(time.Second).String(),
	)

	// Wait for the first midnight.
	select {
	case <-time.After(untilMidnight):
		// Time to sync.
	case <-s.stopCh:
		s.logger.Info("VAT sync scheduler stopped before first midnight sync")
		return
	}

	// Run the midnight sync.
	s.runSync()

	// Set up a 24-hour ticker for subsequent syncs.
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSync()
		case <-s.stopCh:
			s.logger.Info("VAT sync scheduler stopped")
			return
		}
	}
}

// runSync performs a sync and logs the result. It creates a background
// context with a 5-minute timeout for the sync operation.
func (s *Scheduler) runSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.logger.Info("VAT sync scheduler: running scheduled sync")

	result := s.syncer.Sync(ctx)
	if result.Error != nil {
		s.logger.Error("scheduled VAT rate sync failed",
			"error", result.Error,
		)
		// Schedule a retry in 1 hour, up to 3 attempts.
		s.retrySync(3, 1*time.Hour)
		return
	}

	s.logger.Info("scheduled VAT rate sync completed",
		"source", result.Source,
		"rates_loaded", result.RatesLoaded,
		"rates_changed", result.RatesChanged,
	)
}

// retrySync attempts the sync up to maxRetries times with the given delay
// between attempts. It respects the stop signal.
func (s *Scheduler) retrySync(maxRetries int, delay time.Duration) {
	for i := 1; i <= maxRetries; i++ {
		s.logger.Info("VAT sync retry scheduled",
			"attempt", i,
			"max_retries", maxRetries,
			"delay", delay.String(),
		)

		select {
		case <-time.After(delay):
			// Time to retry.
		case <-s.stopCh:
			s.logger.Info("VAT sync retry cancelled: scheduler stopping")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result := s.syncer.Sync(ctx)
		cancel()

		if result.Error == nil {
			s.logger.Info("VAT sync retry succeeded",
				"attempt", i,
				"source", result.Source,
				"rates_loaded", result.RatesLoaded,
				"rates_changed", result.RatesChanged,
			)
			return
		}

		s.logger.Error("VAT sync retry failed",
			"attempt", i,
			"error", result.Error,
		)
	}

	s.logger.Error("all VAT sync retries exhausted",
		"max_retries", maxRetries,
	)
}

// durationUntilNextMidnightUTC calculates the time remaining until the next
// midnight UTC from the current time.
func durationUntilNextMidnightUTC() time.Duration {
	now := time.Now().UTC()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return nextMidnight.Sub(now)
}
