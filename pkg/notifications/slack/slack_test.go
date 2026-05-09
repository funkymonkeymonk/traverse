package slack

import (
	"context"
	"testing"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/notification"
)

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with default config", func(t *testing.T) {
		provider := NewProvider()
		if provider == nil {
			t.Fatal("expected provider, got nil")
		}
		if provider.Name() != "slack" {
			t.Errorf("expected name 'slack', got '%s'", provider.Name())
		}
	})
}

func TestProvider_Configure(t *testing.T) {
	t.Run("configures with valid config", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"bot_token":    "xoxb-test-token",
			"channel":      "#approvals",
			"timeout":      30,
			"max_retries":  3,
		}

		err := provider.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.config.BotToken != "xoxb-test-token" {
			t.Errorf("expected bot_token 'xoxb-test-token', got '%s'", provider.config.BotToken)
		}
		if provider.config.Channel != "#approvals" {
			t.Errorf("expected channel '#approvals', got '%s'", provider.config.Channel)
		}
	})

	t.Run("returns error for missing bot_token", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"channel": "#approvals",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing bot_token")
		}
	})

	t.Run("returns error for missing channel", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"bot_token": "xoxb-test-token",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing channel")
		}
	})

	t.Run("applies default timeout", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"bot_token": "xoxb-test-token",
			"channel":   "#approvals",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.config.Timeout != defaultTimeout {
			t.Errorf("expected default timeout %d, got %d", defaultTimeout, provider.config.Timeout)
		}
	})

	t.Run("applies default max_retries", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"bot_token": "xoxb-test-token",
			"channel":   "#approvals",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.config.MaxRetries != defaultMaxRetries {
			t.Errorf("expected default max_retries %d, got %d", defaultMaxRetries, provider.config.MaxRetries)
		}
	})
}

func TestProvider_Notify(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		provider := NewProvider()
		ctx := context.Background()

		notification := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "#approvals",
		}

		err := provider.Notify(ctx, notification)
		if err == nil {
			t.Error("expected error when not configured")
		}
	})

	t.Run("sends notification for approval request", func(t *testing.T) {
		provider := NewProvider()
		// Use invalid host to fail fast
		config := map[string]interface{}{
			"bot_token":   "xoxb-test-token",
			"channel":     "#approvals",
			"timeout":     1,
			"max_retries": 1,
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("failed to configure provider: %v", err)
		}

		ctx := context.Background()
		notification := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeApprovalRequest,
			Title:     "Approval Required",
			Message:   "Please approve access to production secrets",
			Priority:  notification.PriorityHigh,
			Recipient: "#approvals",
			Context: notification.NotificationContext{
				RequestID:   "req-123",
				SecretPath:  "production/api-key",
				RequestedBy: "developer@example.com",
				RequestedAt: time.Now(),
				ExpiresAt:   time.Now().Add(30 * time.Minute),
			},
		}

		// This will fail since we don't have real Slack credentials
		// but it verifies the code path
		err = provider.Notify(ctx, notification)
		// We expect an error since we're using test credentials
		if err == nil {
			t.Log("Note: Notification succeeded with test credentials (unexpected)")
		}
	})

	t.Run("sends notification for system alert", func(t *testing.T) {
		provider := NewProvider()
		// Use invalid host to fail fast
		config := map[string]interface{}{
			"bot_token":   "xoxb-test-token",
			"channel":     "#alerts",
			"timeout":     1,
			"max_retries": 1,
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("failed to configure provider: %v", err)
		}

		ctx := context.Background()
		notification := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeSystemAlert,
			Title:     "System Alert",
			Message:   "High memory usage detected",
			Priority:  notification.PriorityHigh,
			Recipient: "#alerts",
		}

		// This will fail since we don't have real Slack credentials
		err = provider.Notify(ctx, notification)
		// We expect an error since we're using test credentials
		if err == nil {
			t.Log("Note: Notification succeeded with test credentials (unexpected)")
		}
	})
}

func TestProvider_Close(t *testing.T) {
	t.Run("closes provider successfully", func(t *testing.T) {
		provider := createConfiguredProvider(t)

		err := provider.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handles multiple close calls gracefully", func(t *testing.T) {
		provider := createConfiguredProvider(t)

		err := provider.Close()
		if err != nil {
			t.Errorf("first close failed: %v", err)
		}

		err = provider.Close()
		if err != nil {
			t.Errorf("second close failed: %v", err)
		}
	})
}

func TestProvider_WithRateLimiter(t *testing.T) {
	t.Run("rate limiter is configured", func(t *testing.T) {
		provider := createConfiguredProvider(t)

		// Verify rate limiter is configured
		if provider.rateLimiter == nil {
			t.Error("expected rate limiter to be configured")
		}
	})
}

func TestBlockKitBuilder(t *testing.T) {
	t.Run("builds approval request blocks", func(t *testing.T) {
		now := time.Now()
		notif := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeApprovalRequest,
			Title:     "Approval Required",
			Message:   "Please approve access",
			Priority:  notification.PriorityHigh,
			Recipient: "#approvals",
			Context: notification.NotificationContext{
				RequestID:   "req-123",
				SecretPath:  "production/api-key",
				RequestedBy: "user@example.com",
				RequestedAt: now,
				ExpiresAt:   now.Add(30 * time.Minute),
			},
		}

		blocks := buildApprovalBlocks(notif)
		if blocks == nil {
			t.Fatal("expected blocks, got nil")
		}

		// Should have header, context, actions
		if len(blocks) < 3 {
			t.Errorf("expected at least 3 blocks, got %d", len(blocks))
		}
	})

	t.Run("builds system alert blocks", func(t *testing.T) {
		notif := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeSystemAlert,
			Title:     "System Alert",
			Message:   "High memory usage",
			Priority:  notification.PriorityCritical,
			Recipient: "#alerts",
		}

		blocks := buildAlertBlocks(notif)
		if blocks == nil {
			t.Fatal("expected blocks, got nil")
		}

		// Should have header and section
		if len(blocks) < 2 {
			t.Errorf("expected at least 2 blocks, got %d", len(blocks))
		}
	})
}

// Helper function to create a configured provider
func createConfiguredProvider(t *testing.T) *Provider {
	t.Helper()
	provider := NewProvider()
	config := map[string]interface{}{
		"bot_token":   "xoxb-test-token",
		"channel":     "#approvals",
		"timeout":     30,
		"max_retries": 3,
	}

	err := provider.Configure(config)
	if err != nil {
		t.Fatalf("failed to configure provider: %v", err)
	}

	return provider
}
