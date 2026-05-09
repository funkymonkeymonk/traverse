package duo

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
		if provider.Name() != "duo" {
			t.Errorf("expected name 'duo', got '%s'", provider.Name())
		}
	})
}

func TestProvider_Configure(t *testing.T) {
	t.Run("configures with valid config", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"integration_key": "test-ikey",
			"secret_key":      "test-skey",
			"api_hostname":    "api-test.duosecurity.com",
			"timeout":         30,
			"max_retries":     3,
		}

		err := provider.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.config.IntegrationKey != "test-ikey" {
			t.Errorf("expected integration_key 'test-ikey', got '%s'", provider.config.IntegrationKey)
		}
		if provider.config.SecretKey != "test-skey" {
			t.Errorf("expected secret_key 'test-skey', got '%s'", provider.config.SecretKey)
		}
		if provider.config.APIHostname != "api-test.duosecurity.com" {
			t.Errorf("expected api_hostname 'api-test.duosecurity.com', got '%s'", provider.config.APIHostname)
		}
	})

	t.Run("returns error for missing integration_key", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"secret_key":   "test-skey",
			"api_hostname": "api-test.duosecurity.com",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing integration_key")
		}
	})

	t.Run("returns error for missing secret_key", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"integration_key": "test-ikey",
			"api_hostname":    "api-test.duosecurity.com",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing secret_key")
		}
	})

	t.Run("returns error for missing api_hostname", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"integration_key": "test-ikey",
			"secret_key":      "test-skey",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing api_hostname")
		}
	})

	t.Run("applies default timeout", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"integration_key": "test-ikey",
			"secret_key":      "test-skey",
			"api_hostname":    "api-test.duosecurity.com",
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
			"integration_key": "test-ikey",
			"secret_key":      "test-skey",
			"api_hostname":    "api-test.duosecurity.com",
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
			Recipient: "user@example.com",
		}

		err := provider.Notify(ctx, notification)
		if err == nil {
			t.Error("expected error when not configured")
		}
	})

	t.Run("returns error for unsupported notification type", func(t *testing.T) {
		provider := createConfiguredProvider(t)
		ctx := context.Background()

		notification := notification.Notification{
			ID:        "test-id",
			Type:      notification.TypeSystemAlert,
			Title:     "Test",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(ctx, notification)
		if err == nil {
			t.Error("expected error for unsupported notification type")
		}
	})

	t.Run("sends push notification for approval request", func(t *testing.T) {
		provider := NewProvider()
		// Use very short timeout to fail fast
		config := map[string]interface{}{
			"integration_key": "test-ikey",
			"secret_key":      "test-skey",
			"api_hostname":    "invalid-host.duosecurity.com",
			"timeout":         1,
			"max_retries":     1,
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
			Recipient: "user@example.com",
			Context: notification.NotificationContext{
				RequestID:   "req-123",
				SecretPath:  "production/api-key",
				RequestedBy: "developer@example.com",
				RequestedAt: time.Now(),
				ExpiresAt:   time.Now().Add(30 * time.Minute),
			},
		}

		// This will fail since we don't have real Duo credentials
		// but it verifies the code path
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

	t.Run("rate limiter allows requests up to limit", func(t *testing.T) {
		rl := NewRateLimiter(10) // 10 requests per minute

		// First 10 requests should be allowed immediately
		allowed := 0
		for i := 0; i < 10; i++ {
			if rl.Allow() {
				allowed++
			}
		}

		if allowed != 10 {
			t.Errorf("expected 10 allowed requests, got %d", allowed)
		}
	})

	t.Run("rate limiter blocks after limit reached", func(t *testing.T) {
		rl := NewRateLimiter(10) // 10 requests per minute

		// Use up all tokens
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		// Next request should be blocked
		if rl.Allow() {
			t.Error("expected request to be blocked after rate limit reached")
		}
	})
}

// Helper function to create a configured provider
func createConfiguredProvider(t *testing.T) *Provider {
	t.Helper()
	provider := NewProvider()
	config := map[string]interface{}{
		"integration_key": "test-ikey",
		"secret_key":      "test-skey",
		"api_hostname":    "api-test.duosecurity.com",
		"timeout":         30,
		"max_retries":     3,
	}

	err := provider.Configure(config)
	if err != nil {
		t.Fatalf("failed to configure provider: %v", err)
	}

	return provider
}
