package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockNotifierWithErrors struct {
	mockNotifier
	failOnNotify bool
}

func (m *mockNotifierWithErrors) Notify(ctx context.Context, notification Notification) error {
	if m.failOnNotify {
		return ErrNotificationFailed
	}
	return m.mockNotifier.Notify(ctx, notification)
}

func TestNewManager(t *testing.T) {
	t.Run("creates manager with default config", func(t *testing.T) {
		manager := NewManager(Config{})
		if manager == nil {
			t.Fatal("expected manager, got nil")
		}
	})
}

func TestManager_RegisterProvider(t *testing.T) {
	t.Run("registers provider successfully", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "test-notifier"}

		err := manager.RegisterProvider("test", notifier)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		providers := manager.ListProviders()
		found := false
		for _, name := range providers {
			if name == "test" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'test' to be in registered providers")
		}
	})

	t.Run("returns error for duplicate registration", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "test-notifier"}

		err := manager.RegisterProvider("test", notifier)
		if err != nil {
			t.Fatalf("first registration failed: %v", err)
		}

		err = manager.RegisterProvider("test", notifier)
		if err == nil {
			t.Error("expected error for duplicate registration")
		}
	})

	t.Run("returns error for nil provider", func(t *testing.T) {
		manager := NewManager(Config{})

		err := manager.RegisterProvider("test", nil)
		if err == nil {
			t.Error("expected error for nil provider")
		}
	})
}

func TestManager_Send(t *testing.T) {
	t.Run("sends notification to single provider", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "test-notifier"}

		err := manager.RegisterProvider("test", notifier)
		if err != nil {
			t.Fatalf("registration failed: %v", err)
		}

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err = manager.Send(ctx, notification)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(notifier.notifications) != 1 {
			t.Errorf("expected 1 notification, got %d", len(notifier.notifications))
		}
	})

	t.Run("sends notification to multiple providers", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifier{name: "notifier1"}
		notifier2 := &mockNotifier{name: "notifier2"}

		manager.RegisterProvider("n1", notifier1)
		manager.RegisterProvider("n2", notifier2)

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.Send(ctx, notification)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(notifier1.notifications) != 1 {
			t.Errorf("expected notifier1 to have 1 notification, got %d", len(notifier1.notifications))
		}
		if len(notifier2.notifications) != 1 {
			t.Errorf("expected notifier2 to have 1 notification, got %d", len(notifier2.notifications))
		}
	})

	t.Run("returns error when no providers registered", func(t *testing.T) {
		manager := NewManager(Config{})

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.Send(ctx, notification)
		if err == nil {
			t.Error("expected error when no providers registered")
		}
	})

	t.Run("continues on partial failure with multi-provider", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifierWithErrors{
			mockNotifier: mockNotifier{name: "notifier1"},
			failOnNotify: true,
		}
		notifier2 := &mockNotifier{name: "notifier2"}

		manager.RegisterProvider("n1", notifier1)
		manager.RegisterProvider("n2", notifier2)

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.Send(ctx, notification)
		if err == nil {
			t.Error("expected error when one provider fails")
		}

		if len(notifier2.notifications) != 1 {
			t.Errorf("expected notifier2 to still receive notification, got %d", len(notifier2.notifications))
		}
	})
}

func TestManager_SendWithPriority(t *testing.T) {
	t.Run("filters providers by minimum priority", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifier{name: "notifier1"}
		notifier2 := &mockNotifier{name: "notifier2"}

		manager.RegisterProvider("n1", notifier1)
		manager.RegisterProvider("n2", notifier2)

		lowPriorityNotification := Notification{
			ID:        "test-id",
			Type:      TypeSystemAlert,
			Title:     "Low Priority",
			Message:   "Test message",
			Priority:  PriorityLow,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		manager.SendWithPriority(ctx, lowPriorityNotification, PriorityMedium)

		if len(notifier1.notifications) != 0 {
			t.Errorf("expected notifier1 to receive 0 notifications, got %d", len(notifier1.notifications))
		}
	})

	t.Run("sends to all providers when priority meets threshold", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifier{name: "notifier1"}

		manager.RegisterProvider("n1", notifier1)

		highPriorityNotification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "High Priority",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.SendWithPriority(ctx, highPriorityNotification, PriorityMedium)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(notifier1.notifications) != 1 {
			t.Errorf("expected 1 notification, got %d", len(notifier1.notifications))
		}
	})
}

func TestManager_SendToProvider(t *testing.T) {
	t.Run("sends to specific provider", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifier{name: "notifier1"}
		notifier2 := &mockNotifier{name: "notifier2"}

		manager.RegisterProvider("n1", notifier1)
		manager.RegisterProvider("n2", notifier2)

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.SendToProvider(ctx, "n1", notification)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(notifier1.notifications) != 1 {
			t.Errorf("expected notifier1 to have 1 notification, got %d", len(notifier1.notifications))
		}
		if len(notifier2.notifications) != 0 {
			t.Errorf("expected notifier2 to have 0 notifications, got %d", len(notifier2.notifications))
		}
	})

	t.Run("returns error for unknown provider", func(t *testing.T) {
		manager := NewManager(Config{})

		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
		}

		ctx := context.Background()
		err := manager.SendToProvider(ctx, "unknown", notification)
		if !errors.Is(err, ErrNotifierNotFound) {
			t.Errorf("expected ErrNotifierNotFound, got %v", err)
		}
	})
}

func TestManager_Close(t *testing.T) {
	t.Run("closes all providers", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier1 := &mockNotifier{name: "notifier1"}
		notifier2 := &mockNotifier{name: "notifier2"}

		manager.RegisterProvider("n1", notifier1)
		manager.RegisterProvider("n2", notifier2)

		err := manager.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !notifier1.closeCalled {
			t.Error("expected notifier1 to be closed")
		}
		if !notifier2.closeCalled {
			t.Error("expected notifier2 to be closed")
		}
	})

	t.Run("handles multiple close calls gracefully", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "notifier"}

		manager.RegisterProvider("n1", notifier)

		err := manager.Close()
		if err != nil {
			t.Errorf("first close failed: %v", err)
		}

		err = manager.Close()
		if err != nil {
			t.Errorf("second close failed: %v", err)
		}
	})
}

func TestManager_GetProviderStatus(t *testing.T) {
	t.Run("returns status for all providers", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "notifier"}

		manager.RegisterProvider("n1", notifier)

		statuses := manager.GetProviderStatus()
		if len(statuses) != 1 {
			t.Errorf("expected 1 status, got %d", len(statuses))
		}

		if _, ok := statuses["n1"]; !ok {
			t.Error("expected status for 'n1' provider")
		}
	})
}

func TestManager_ConfigureProvider(t *testing.T) {
	t.Run("configures provider with settings", func(t *testing.T) {
		manager := NewManager(Config{})
		notifier := &mockNotifier{name: "notifier"}

		manager.RegisterProvider("n1", notifier)

		config := map[string]interface{}{
			"webhook_url": "https://example.com/webhook",
			"timeout":     30,
		}

		err := manager.ConfigureProvider("n1", config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !notifier.configured {
			t.Error("expected notifier to be configured")
		}
	})

	t.Run("returns error for unknown provider", func(t *testing.T) {
		manager := NewManager(Config{})

		config := map[string]interface{}{"key": "value"}
		err := manager.ConfigureProvider("unknown", config)
		if !errors.Is(err, ErrNotifierNotFound) {
			t.Errorf("expected ErrNotifierNotFound, got %v", err)
		}
	})
}

func TestNotificationResult(t *testing.T) {
	t.Run("Result tracks successes and failures", func(t *testing.T) {
		result := NotificationResult{
			NotificationID: "notif-123",
			SentAt:         time.Now(),
			SuccessCount:   2,
			FailureCount:   1,
			Errors: map[string]string{
				"provider1": "connection timeout",
			},
		}

		if result.NotificationID != "notif-123" {
			t.Errorf("expected notification_id 'notif-123', got '%s'", result.NotificationID)
		}
		if result.SuccessCount != 2 {
			t.Errorf("expected 2 successes, got %d", result.SuccessCount)
		}
		if result.FailureCount != 1 {
			t.Errorf("expected 1 failure, got %d", result.FailureCount)
		}
	})

	t.Run("AllSucceeded returns true when no failures", func(t *testing.T) {
		result := NotificationResult{
			SuccessCount: 2,
			FailureCount: 0,
		}

		if !result.AllSucceeded() {
			t.Error("expected AllSucceeded to be true")
		}
	})

	t.Run("AllSucceeded returns false when failures exist", func(t *testing.T) {
		result := NotificationResult{
			SuccessCount: 1,
			FailureCount: 1,
		}

		if result.AllSucceeded() {
			t.Error("expected AllSucceeded to be false")
		}
	})

	t.Run("HasPartialSuccess returns true when some succeed", func(t *testing.T) {
		result := NotificationResult{
			SuccessCount: 1,
			FailureCount: 1,
		}

		if !result.HasPartialSuccess() {
			t.Error("expected HasPartialSuccess to be true")
		}
	})
}
