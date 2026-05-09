package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Storage defines the interface for cleanup operations
type Storage interface {
	ListExpiredRequests() ([]*Request, error)
	ListOldRequests(retention time.Duration) ([]*Request, error)
	DeleteRequest(id string) error
	GetStats() (*Stats, error)
}

// Request represents a request in storage
type Request struct {
	ID         string
	Status     string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	ResolvedAt time.Time
}

// Stats contains statistics about requests
type Stats struct {
	TotalRequests   int
	PendingRequests int
	ExpiredRequests int
}

// CleanupScheduler periodically cleans up expired and old requests
type CleanupScheduler struct {
	storage          Storage
	interval         time.Duration
	retentionPeriod  time.Duration
	ticker           *time.Ticker
	stopCh           chan struct{}
	wg               sync.WaitGroup
	running          bool
	mu               sync.Mutex
	lastCleanupTime  time.Time
	lastCleanupCount int
}

// NewCleanupScheduler creates a new cleanup scheduler
func NewCleanupScheduler(storage Storage, interval time.Duration) *CleanupScheduler {
	return &CleanupScheduler{
		storage:         storage,
		interval:        interval,
		retentionPeriod: 30 * 24 * time.Hour, // Default 30 days
		stopCh:          make(chan struct{}),
	}
}

// NewCleanupSchedulerWithRetention creates a scheduler with custom retention
func NewCleanupSchedulerWithRetention(storage Storage, interval, retention time.Duration) *CleanupScheduler {
	return &CleanupScheduler{
		storage:         storage,
		interval:        interval,
		retentionPeriod: retention,
		stopCh:          make(chan struct{}),
	}
}

// Start begins the cleanup scheduler
func (cs *CleanupScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.running {
		return fmt.Errorf("scheduler is already running")
	}

	cs.running = true
	cs.ticker = time.NewTicker(cs.interval)

	cs.wg.Add(1)
	go cs.run()

	return nil
}

// Stop halts the cleanup scheduler
func (cs *CleanupScheduler) Stop() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.running {
		return nil
	}

	cs.running = false
	close(cs.stopCh)

	if cs.ticker != nil {
		cs.ticker.Stop()
	}

	cs.wg.Wait()
	return nil
}

// run is the main loop that runs cleanup at intervals
func (cs *CleanupScheduler) run() {
	defer cs.wg.Done()

	// Run cleanup immediately on start
	cs.cleanup()

	for {
		select {
		case <-cs.ticker.C:
			cs.cleanup()
		case <-cs.stopCh:
			return
		}
	}
}

// cleanup performs the actual cleanup operations
func (cs *CleanupScheduler) cleanup() {
	// Cleanup expired requests
	_, _ = cs.CleanupExpiredRequests()

	// Cleanup old resolved requests
	_, _ = cs.CleanupOldRequests()
}

// CleanupExpiredRequests removes all expired pending requests
func (cs *CleanupScheduler) CleanupExpiredRequests() (int, error) {
	expiredRequests, err := cs.storage.ListExpiredRequests()
	if err != nil {
		return 0, fmt.Errorf("failed to list expired requests: %w", err)
	}

	count := 0
	for _, req := range expiredRequests {
		if err := cs.storage.DeleteRequest(req.ID); err != nil {
			// Log error but continue with other requests
			continue
		}
		count++
	}

	cs.mu.Lock()
	cs.lastCleanupTime = time.Now()
	cs.lastCleanupCount = count
	cs.mu.Unlock()

	return count, nil
}

// CleanupOldRequests removes resolved requests older than the retention period
func (cs *CleanupScheduler) CleanupOldRequests() (int, error) {
	oldRequests, err := cs.storage.ListOldRequests(cs.retentionPeriod)
	if err != nil {
		return 0, fmt.Errorf("failed to list old requests: %w", err)
	}

	count := 0
	for _, req := range oldRequests {
		if err := cs.storage.DeleteRequest(req.ID); err != nil {
			// Log error but continue with other requests
			continue
		}
		count++
	}

	return count, nil
}

// GetStats returns statistics about requests in storage
func (cs *CleanupScheduler) GetStats() (*Stats, error) {
	return cs.storage.GetStats()
}

// SetRetentionPeriod updates the retention period for old requests
func (cs *CleanupScheduler) SetRetentionPeriod(period time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.retentionPeriod = period
}

// GetRetentionPeriod returns the current retention period
func (cs *CleanupScheduler) GetRetentionPeriod() time.Duration {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.retentionPeriod
}

// IsRunning returns true if the scheduler is currently running
func (cs *CleanupScheduler) IsRunning() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.running
}

// GetLastCleanupInfo returns information about the last cleanup run
func (cs *CleanupScheduler) GetLastCleanupInfo() (time.Time, int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.lastCleanupTime, cs.lastCleanupCount
}

// RunOnce performs a single cleanup cycle without starting the scheduler
func (cs *CleanupScheduler) RunOnce() (int, error) {
	expiredCount, err := cs.CleanupExpiredRequests()
	if err != nil {
		return 0, err
	}

	oldCount, err := cs.CleanupOldRequests()
	if err != nil {
		return expiredCount, err
	}

	return expiredCount + oldCount, nil
}

// CleanupResult contains the results of a cleanup operation
type CleanupResult struct {
	ExpiredCleaned int
	OldCleaned     int
	Errors         []error
}

// CleanupAll performs comprehensive cleanup and returns detailed results
func (cs *CleanupScheduler) CleanupAll(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{
		Errors: []error{},
	}

	// Cleanup expired requests
	expiredRequests, err := cs.storage.ListExpiredRequests()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to list expired requests: %w", err))
	} else {
		for _, req := range expiredRequests {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
				if err := cs.storage.DeleteRequest(req.ID); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to delete request %s: %w", req.ID, err))
				} else {
					result.ExpiredCleaned++
				}
			}
		}
	}

	// Cleanup old requests
	oldRequests, err := cs.storage.ListOldRequests(cs.retentionPeriod)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to list old requests: %w", err))
	} else {
		for _, req := range oldRequests {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
				if err := cs.storage.DeleteRequest(req.ID); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to delete request %s: %w", req.ID, err))
				} else {
					result.OldCleaned++
				}
			}
		}
	}

	return result, nil
}
