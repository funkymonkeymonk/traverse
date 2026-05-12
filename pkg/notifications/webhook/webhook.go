// Package webhook implements a generic webhook notification provider
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/notification"
)

const (
	providerName      = "webhook"
	defaultTimeout    = 30
	defaultRetryCount = 3
	defaultRetryBackoff = 5
)

// Common errors
var (
	ErrNotConfigured      = errors.New("webhook provider not configured")
	ErrMissingURL         = errors.New("url is required")
	ErrInvalidMethod      = errors.New("method must be POST or PUT")
	ErrWebhookFailed      = errors.New("webhook delivery failed")
	ErrInvalidStatusCode  = errors.New("webhook returned non-success status code")
)

// Config holds webhook provider configuration
type Config struct {
	// URL is the webhook endpoint
	URL string

	// Method is the HTTP method (POST or PUT)
	Method string

	// Headers are custom headers to include
	Headers map[string]string

	// Timeout for HTTP requests in seconds
	Timeout int

	// RetryCount is the number of retries on failure
	RetryCount int

	// RetryBackoff is the delay between retries in seconds
	RetryBackoff int

	// Secret for HMAC signature generation
	Secret string

	// SignatureHeader is the header name for the HMAC signature
	SignatureHeader string
}

// Provider implements the notification.Notifier interface for webhooks
type Provider struct {
	config     *Config
	configured bool
	client     *http.Client
}

// WebhookPayload represents the payload sent to webhooks
type WebhookPayload struct {
	// Notification is the notification data
	Notification notification.Notification `json:"notification"`

	// Timestamp is when the webhook was sent
	Timestamp time.Time `json:"timestamp"`

	// Signature is the HMAC signature (if configured)
	Signature string `json:"signature,omitempty"`
}

// NewProvider creates a new webhook provider
func NewProvider() *Provider {
	return &Provider{
		configured: false,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return providerName
}

// Configure initializes the provider with the given configuration
func (p *Provider) Configure(config map[string]interface{}) error {
	cfg := &Config{}

	// Parse URL (required)
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return ErrMissingURL
	}
	cfg.URL = url

	// Parse method with default
	cfg.Method = "POST"
	if method, ok := config["method"].(string); ok && method != "" {
		if method != "POST" && method != "PUT" {
			return ErrInvalidMethod
		}
		cfg.Method = method
	}

	// Parse timeout with default
	cfg.Timeout = defaultTimeout
	if timeout, ok := config["timeout"].(int); ok && timeout > 0 {
		cfg.Timeout = timeout
	}

	// Parse retry count with default
	cfg.RetryCount = defaultRetryCount
	if retryCount, ok := config["retry_count"].(int); ok && retryCount >= 0 {
		cfg.RetryCount = retryCount
	}

	// Parse retry backoff with default
	cfg.RetryBackoff = defaultRetryBackoff
	if retryBackoff, ok := config["retry_backoff"].(int); ok && retryBackoff > 0 {
		cfg.RetryBackoff = retryBackoff
	}

	// Parse headers
	if headers, ok := config["headers"].(map[string]string); ok {
		cfg.Headers = headers
	}

	// Parse secret for signature
	if secret, ok := config["secret"].(string); ok {
		cfg.Secret = secret
	}

	// Parse signature header name
	if sigHeader, ok := config["signature_header"].(string); ok && sigHeader != "" {
		cfg.SignatureHeader = sigHeader
	} else {
		cfg.SignatureHeader = "X-Webhook-Signature"
	}

	// Initialize HTTP client
	p.client = &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	p.config = cfg
	p.configured = true

	return nil
}

// Notify sends a notification via webhook
func (p *Provider) Notify(ctx context.Context, notif notification.Notification) error {
	if !p.configured {
		return notification.ErrNotifierNotConfigured
	}

	// Create payload
	payload := NewWebhookPayload(notif)

	// Marshal payload
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send with retry
	var lastErr error
	for attempt := 0; attempt <= p.config.RetryCount; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(time.Duration(p.config.RetryBackoff) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := p.sendWebhook(ctx, body)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("%w: %v", ErrWebhookFailed, lastErr)
}

// sendWebhook sends a single webhook request
func (p *Provider) sendWebhook(ctx context.Context, body []byte) error {
	// Create request
	req, err := http.NewRequestWithContext(ctx, p.config.Method, p.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range p.config.Headers {
		req.Header.Set(key, value)
	}

	// Add signature if secret is configured
	if p.config.Secret != "" {
		signature := generateHMACSignature(body, p.config.Secret)
		req.Header.Set(p.config.SignatureHeader, signature)
	}

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %d - %s", ErrInvalidStatusCode, resp.StatusCode, string(body))
	}

	return nil
}

// Close cleans up provider resources
func (p *Provider) Close() error {
	p.configured = false
	p.config = nil
	p.client = nil
	return nil
}

// NewWebhookPayload creates a new webhook payload from a notification
func NewWebhookPayload(notif notification.Notification) WebhookPayload {
	return WebhookPayload{
		Notification: notif,
		Timestamp:    time.Now(),
	}
}

// generateHMACSignature generates an HMAC-SHA256 signature
func generateHMACSignature(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Ensure Provider implements Notifier interface
var _ notification.Notifier = (*Provider)(nil)
