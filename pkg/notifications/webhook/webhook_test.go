package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/notification"
)

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with defaults", func(t *testing.T) {
		provider := NewProvider()

		if provider == nil {
			t.Fatal("expected provider to be created")
		}
		if provider.Name() != "webhook" {
			t.Errorf("expected name 'webhook', got '%s'", provider.Name())
		}
		if provider.configured {
			t.Error("expected provider to be unconfigured initially")
		}
	})
}

func TestConfigure(t *testing.T) {
	t.Run("configures with valid settings", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url":     "https://example.com/webhook",
			"method":  "POST",
			"timeout": 30,
			"headers": map[string]string{
				"Authorization": "Bearer token123",
			},
			"retry_count":    3,
			"retry_backoff":  5,
			"secret":         "webhook-secret",
			"signature_header": "X-Webhook-Signature",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !provider.configured {
			t.Error("expected provider to be configured")
		}
		if provider.config.URL != "https://example.com/webhook" {
			t.Errorf("expected URL 'https://example.com/webhook', got '%s'", provider.config.URL)
		}
		if provider.config.Method != "POST" {
			t.Errorf("expected method 'POST', got '%s'", provider.config.Method)
		}
		if provider.config.Timeout != 30 {
			t.Errorf("expected timeout 30, got %d", provider.config.Timeout)
		}
		if provider.config.RetryCount != 3 {
			t.Errorf("expected retry_count 3, got %d", provider.config.RetryCount)
		}
	})

	t.Run("requires URL", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"method": "POST",
		}

		err := provider.Configure(config)
		if err != ErrMissingURL {
			t.Errorf("expected ErrMissingURL, got: %v", err)
		}
	})

	t.Run("sets default method to POST", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url": "https://example.com/webhook",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider.config.Method != "POST" {
			t.Errorf("expected default method 'POST', got '%s'", provider.config.Method)
		}
	})

	t.Run("sets default timeout", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url": "https://example.com/webhook",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider.config.Timeout != defaultTimeout {
			t.Errorf("expected default timeout %d, got %d", defaultTimeout, provider.config.Timeout)
		}
	})

	t.Run("sets default retry settings", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url": "https://example.com/webhook",
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider.config.RetryCount != defaultRetryCount {
			t.Errorf("expected default retry_count %d, got %d", defaultRetryCount, provider.config.RetryCount)
		}
		if provider.config.RetryBackoff != defaultRetryBackoff {
			t.Errorf("expected default retry_backoff %d, got %d", defaultRetryBackoff, provider.config.RetryBackoff)
		}
	})

	t.Run("accepts only POST or PUT methods", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url":    "https://example.com/webhook",
			"method": "GET",
		}

		err := provider.Configure(config)
		if err != ErrInvalidMethod {
			t.Errorf("expected ErrInvalidMethod, got: %v", err)
		}
	})

	t.Run("parses headers correctly", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"url":    "https://example.com/webhook",
			"method": "POST",
			"headers": map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer token123",
			},
		}

		err := provider.Configure(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(provider.config.Headers) != 2 {
			t.Errorf("expected 2 headers, got %d", len(provider.config.Headers))
		}
		if provider.config.Headers["Content-Type"] != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", provider.config.Headers["Content-Type"])
		}
	})
}

func TestNotify(t *testing.T) {
	t.Run("sends POST webhook successfully", func(t *testing.T) {
		var receivedBody []byte
		var receivedHeaders http.Header

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":    server.URL,
			"method": "POST",
			"headers": map[string]string{
				"X-Custom-Header": "custom-value",
			},
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if receivedHeaders.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", receivedHeaders.Get("Content-Type"))
		}
		if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header 'custom-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
		}

		var payload WebhookPayload
		json.Unmarshal(receivedBody, &payload)
		if payload.Notification.ID != "notif-123" {
			t.Errorf("expected notification ID 'notif-123', got '%s'", payload.Notification.ID)
		}
	})

	t.Run("sends PUT webhook successfully", func(t *testing.T) {
		var requestMethod string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":    server.URL,
			"method": "PUT",
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if requestMethod != "PUT" {
			t.Errorf("expected method 'PUT', got '%s'", requestMethod)
		}
	})

	t.Run("includes HMAC signature when secret is configured", func(t *testing.T) {
		var receivedSignature string
		var receivedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedSignature = r.Header.Get("X-Webhook-Signature")
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":              server.URL,
			"method":           "POST",
			"secret":           "webhook-secret",
			"signature_header": "X-Webhook-Signature",
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if receivedSignature == "" {
			t.Fatal("expected signature header to be set")
		}

		// Verify signature
		expectedSig := generateHMACSignature(receivedBody, "webhook-secret")
		if receivedSignature != expectedSig {
			t.Errorf("signature mismatch: expected '%s', got '%s'", expectedSig, receivedSignature)
		}
	})

	t.Run("retries on server error", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":           server.URL,
			"method":        "POST",
			"retry_count":   3,
			"retry_backoff": 1,
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if attemptCount != 3 {
			t.Errorf("expected 3 attempts, got %d", attemptCount)
		}
	})

	t.Run("returns error after max retries exceeded", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":           server.URL,
			"method":        "POST",
			"retry_count":   2,
			"retry_backoff": 1,
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err == nil {
			t.Fatal("expected error after max retries")
		}
		if !errors.Is(err, ErrWebhookFailed) {
			t.Errorf("expected ErrWebhookFailed, got: %v", err)
		}
	})

	t.Run("returns error when not configured", func(t *testing.T) {
		provider := NewProvider()

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err != notification.ErrNotifierNotConfigured {
			t.Errorf("expected ErrNotifierNotConfigured, got: %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":     server.URL,
			"method":  "POST",
			"timeout": 60,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(ctx, notif)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("handles non-2xx status codes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid payload"}`))
		}))
		defer server.Close()

		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":         server.URL,
			"method":      "POST",
			"retry_count": 1,
		})

		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
		}

		err := provider.Notify(context.Background(), notif)
		if err == nil {
			t.Fatal("expected error for non-2xx status")
		}
	})
}

func TestWebhookPayload(t *testing.T) {
	t.Run("payload contains all notification fields", func(t *testing.T) {
		now := time.Now()
		notif := notification.Notification{
			ID:        "notif-123",
			Type:      notification.TypeApprovalRequest,
			Title:     "Test Title",
			Message:   "Test Message",
			Priority:  notification.PriorityHigh,
			Recipient: "user@example.com",
			Context: notification.NotificationContext{
				RequestID:   "req-456",
				SecretPath:  "vault/secret",
				RequestedBy: "requester@example.com",
				RequestedAt: now,
				ExpiresAt:   now.Add(1 * time.Hour),
				Metadata: map[string]string{
					"key": "value",
				},
			},
			Actions: []notification.Action{
				{Type: notification.ActionApprove, Label: "Approve", URL: "https://example.com/approve"},
			},
			CreatedAt: now,
		}

		payload := NewWebhookPayload(notif)

		if payload.Notification.ID != "notif-123" {
			t.Errorf("expected ID 'notif-123', got '%s'", payload.Notification.ID)
		}
		if payload.Notification.Title != "Test Title" {
			t.Errorf("expected Title 'Test Title', got '%s'", payload.Notification.Title)
		}
		if payload.Notification.Context.RequestID != "req-456" {
			t.Errorf("expected RequestID 'req-456', got '%s'", payload.Notification.Context.RequestID)
		}
		if len(payload.Notification.Actions) != 1 {
			t.Errorf("expected 1 action, got %d", len(payload.Notification.Actions))
		}
		if payload.Timestamp.IsZero() {
			t.Error("expected timestamp to be set")
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("closes provider successfully", func(t *testing.T) {
		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url": "https://example.com/webhook",
		})

		err := provider.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.configured {
			t.Error("expected provider to be unconfigured after close")
		}
	})

	t.Run("closes unconfigured provider without error", func(t *testing.T) {
		provider := NewProvider()

		err := provider.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNotifierInterface(t *testing.T) {
	t.Run("implements Notifier interface", func(t *testing.T) {
		var _ notification.Notifier = (*Provider)(nil)
	})
}

func TestTimeout(t *testing.T) {
	t.Run("applies configured timeout", func(t *testing.T) {
		provider := NewProvider()
		provider.Configure(map[string]interface{}{
			"url":     "https://example.com/webhook",
			"timeout": 5,
		})

		if provider.config.Timeout != 5 {
			t.Errorf("expected timeout 5, got %d", provider.config.Timeout)
		}
	})
}
