package scheduler

import (
	"testing"
	"time"
)

// Cleanup Scheduler Tests
// These tests verify expired request cleanup functionality

func TestCleanupScheduler_StartStop(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	// Test: Scheduler can be started
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Expected no error starting scheduler, got: %v", err)
	}

	// Test: Scheduler can be stopped
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Expected no error stopping scheduler, got: %v", err)
	}
}

func TestCleanupScheduler_CleanupExpiredRequests(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	now := time.Now()

	// Add expired requests
	storage.SaveRequest(&MockRequest{
		ID:        "expired-1",
		Status:    "pending",
		ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	})

	storage.SaveRequest(&MockRequest{
		ID:        "expired-2",
		Status:    "pending",
		ExpiresAt: now.Add(-30 * time.Minute),
		CreatedAt: now.Add(-1 * time.Hour),
	})

	// Add non-expired request
	storage.SaveRequest(&MockRequest{
		ID:        "active-1",
		Status:    "pending",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now.Add(-30 * time.Minute),
	})

	// Add already-resolved request (should not be touched)
	storage.SaveRequest(&MockRequest{
		ID:        "approved-1",
		Status:    "approved",
		ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	})

	// Run cleanup
	count, err := scheduler.CleanupExpiredRequests()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have cleaned up 2 expired pending requests
	if count != 2 {
		t.Errorf("Expected 2 expired requests cleaned up, got: %d", count)
	}

	// Verify expired requests were deleted
	if _, err := storage.GetRequest("expired-1"); err == nil {
		t.Error("Expected expired-1 to be deleted")
	}

	if _, err := storage.GetRequest("expired-2"); err == nil {
		t.Error("Expected expired-2 to be deleted")
	}

	// Verify non-expired request still exists
	if _, err := storage.GetRequest("active-1"); err != nil {
		t.Error("Expected active-1 to still exist")
	}

	// Verify approved request still exists (even if expired)
	if _, err := storage.GetRequest("approved-1"); err != nil {
		t.Error("Expected approved-1 to still exist (terminal state)")
	}
}

func TestCleanupScheduler_CleanupOldRequests(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	now := time.Now()

	// Add old resolved requests (older than retention period)
	storage.SaveRequest(&MockRequest{
		ID:         "old-approved",
		Status:     "approved",
		CreatedAt:  now.Add(-30 * 24 * time.Hour), // 30 days old
		ResolvedAt: now.Add(-29 * 24 * time.Hour),
	})

	storage.SaveRequest(&MockRequest{
		ID:         "old-denied",
		Status:     "denied",
		CreatedAt:  now.Add(-30 * 24 * time.Hour),
		ResolvedAt: now.Add(-29 * 24 * time.Hour),
	})

	// Add recent resolved request (within retention period)
	storage.SaveRequest(&MockRequest{
		ID:         "recent-approved",
		Status:     "approved",
		CreatedAt:  now.Add(-1 * 24 * time.Hour),
		ResolvedAt: now.Add(-23 * time.Hour),
	})

	// Set retention to 7 days
	scheduler.SetRetentionPeriod(7 * 24 * time.Hour)

	// Run cleanup for old requests
	count, err := scheduler.CleanupOldRequests()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have cleaned up 2 old resolved requests
	if count != 2 {
		t.Errorf("Expected 2 old requests cleaned up, got: %d", count)
	}

	// Verify old requests were deleted
	if _, err := storage.GetRequest("old-approved"); err == nil {
		t.Error("Expected old-approved to be deleted")
	}

	if _, err := storage.GetRequest("old-denied"); err == nil {
		t.Error("Expected old-denied to be deleted")
	}

	// Verify recent request still exists
	if _, err := storage.GetRequest("recent-approved"); err != nil {
		t.Error("Expected recent-approved to still exist")
	}
}

func TestCleanupScheduler_NoExpiredRequests(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	now := time.Now()

	// Add only active requests
	storage.SaveRequest(&MockRequest{
		ID:        "active-1",
		Status:    "pending",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now.Add(-30 * time.Minute),
	})

	storage.SaveRequest(&MockRequest{
		ID:        "active-2",
		Status:    "pending",
		ExpiresAt: now.Add(2 * time.Hour),
		CreatedAt: now.Add(-1 * time.Hour),
	})

	// Run cleanup
	count, err := scheduler.CleanupExpiredRequests()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have cleaned up 0 requests
	if count != 0 {
		t.Errorf("Expected 0 expired requests, got: %d", count)
	}

	// Verify all requests still exist
	if _, err := storage.GetRequest("active-1"); err != nil {
		t.Error("Expected active-1 to still exist")
	}

	if _, err := storage.GetRequest("active-2"); err != nil {
		t.Error("Expected active-2 to still exist")
	}
}

func TestCleanupScheduler_Schedule(t *testing.T) {
	storage := NewMockStorage()

	// Create scheduler with short interval for testing
	scheduler := NewCleanupScheduler(storage, 100*time.Millisecond)

	now := time.Now()

	// Add expired request
	storage.SaveRequest(&MockRequest{
		ID:        "expired-1",
		Status:    "pending",
		ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	})

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Expected no error starting scheduler, got: %v", err)
	}

	// Wait for at least one cleanup cycle
	time.Sleep(250 * time.Millisecond)

	// Stop scheduler
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Expected no error stopping scheduler, got: %v", err)
	}

	// Verify expired request was cleaned up
	if _, err := storage.GetRequest("expired-1"); err == nil {
		t.Error("Expected expired-1 to be deleted after scheduled cleanup")
	}
}

func TestCleanupScheduler_GetStats(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	now := time.Now()

	// Add requests with various states
	storage.SaveRequest(&MockRequest{
		ID:        "expired-1",
		Status:    "pending",
		ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	})

	storage.SaveRequest(&MockRequest{
		ID:        "active-1",
		Status:    "pending",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now.Add(-30 * time.Minute),
	})

	storage.SaveRequest(&MockRequest{
		ID:     "approved-1",
		Status: "approved",
		CreatedAt: now.Add(-1 * time.Hour),
	})

	stats, err := scheduler.GetStats()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got: %d", stats.TotalRequests)
	}

	if stats.PendingRequests != 2 {
		t.Errorf("Expected 2 pending requests, got: %d", stats.PendingRequests)
	}

	if stats.ExpiredRequests != 1 {
		t.Errorf("Expected 1 expired request, got: %d", stats.ExpiredRequests)
	}
}

func TestCleanupScheduler_ConcurrentCleanup(t *testing.T) {
	storage := NewMockStorage()
	scheduler := NewCleanupScheduler(storage, 1*time.Hour)

	now := time.Now()

	// Add multiple expired requests
	for i := 0; i < 100; i++ {
		storage.SaveRequest(&MockRequest{
			ID:        MockRequestID(i),
			Status:    "pending",
			ExpiresAt: now.Add(-1 * time.Hour),
			CreatedAt: now.Add(-2 * time.Hour),
		})
	}

	// Run cleanup multiple times concurrently
	done := make(chan int, 3)
	for i := 0; i < 3; i++ {
		go func() {
			count, _ := scheduler.CleanupExpiredRequests()
			done <- count
		}()
	}

	// Wait for all goroutines
	totalCleaned := 0
	for i := 0; i < 3; i++ {
		totalCleaned += <-done
	}

	// Total cleaned should be 100 (even with concurrent runs)
	if totalCleaned < 100 {
		t.Errorf("Expected at least 100 total cleanups, got: %d", totalCleaned)
	}

	// Verify all expired requests are gone
	remainingExpired := 0
	for i := 0; i < 100; i++ {
		if _, err := storage.GetRequest(MockRequestID(i)); err == nil {
			remainingExpired++
		}
	}

	if remainingExpired > 0 {
		t.Errorf("Expected 0 remaining expired requests, got: %d", remainingExpired)
	}
}

// Mock implementations for testing

type MockRequest struct {
	ID         string
	Status     string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	ResolvedAt time.Time
}

type MockStorage struct {
	requests map[string]*MockRequest
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		requests: make(map[string]*MockRequest),
	}
}

func (m *MockStorage) SaveRequest(req *MockRequest) error {
	m.requests[req.ID] = req
	return nil
}

func (m *MockStorage) GetRequest(id string) (*MockRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, ErrRequestNotFound
	}
	return req, nil
}

func (m *MockStorage) DeleteRequest(id string) error {
	delete(m.requests, id)
	return nil
}

func (m *MockStorage) ListExpiredRequests() ([]*MockRequest, error) {
	now := time.Now()
	var result []*MockRequest
	for _, req := range m.requests {
		if req.Status == "pending" && req.ExpiresAt.Before(now) {
			result = append(result, req)
		}
	}
	return result, nil
}

func (m *MockStorage) ListOldRequests(retention time.Duration) ([]*MockRequest, error) {
	cutoff := time.Now().Add(-retention)
	var result []*MockRequest
	for _, req := range m.requests {
		if req.Status != "pending" && req.ResolvedAt.Before(cutoff) {
			result = append(result, req)
		}
	}
	return result, nil
}

func (m *MockStorage) GetStats() (*CleanupStats, error) {
	now := time.Now()
	stats := &CleanupStats{}

	for _, req := range m.requests {
		stats.TotalRequests++
		if req.Status == "pending" {
			stats.PendingRequests++
			if req.ExpiresAt.Before(now) {
				stats.ExpiredRequests++
			}
		}
	}

	return stats, nil
}

func MockRequestID(i int) string {
	return "req-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}

type CleanupStats struct {
	TotalRequests   int
	PendingRequests int
	ExpiredRequests int
}

var ErrRequestNotFound = ErrNotFound{Message: "request not found"}

type ErrNotFound struct {
	Message string
}

func (e ErrNotFound) Error() string {
	return e.Message
}
