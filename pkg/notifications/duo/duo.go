// Package duo implements a Duo Push notification provider
package duo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/duosecurity/duo_api_golang"
	"github.com/duosecurity/duo_api_golang/authapi"
	"github.com/funkymonkeymonk/traverse/pkg/notification"
)

const (
	providerName      = "duo"
	defaultTimeout    = 30
	defaultMaxRetries = 3
	defaultRateLimit  = 10 // requests per minute
)

// Common errors
var (
	ErrNotConfigured         = errors.New("duo provider not configured")
	ErrUnsupportedType       = errors.New("unsupported notification type for Duo")
	ErrRateLimitExceeded     = errors.New("rate limit exceeded")
	ErrDuoAPIError           = errors.New("duo api error")
	ErrMissingIntegrationKey = errors.New("integration_key is required")
	ErrMissingSecretKey      = errors.New("secret_key is required")
	ErrMissingAPIHostname    = errors.New("api_hostname is required")
)

// Config holds Duo provider configuration
type Config struct {
	// IntegrationKey is the Duo integration key
	IntegrationKey string

	// SecretKey is the Duo secret key
	SecretKey string

	// APIHostname is the Duo API hostname (e.g., api-xxx.duosecurity.com)
	APIHostname string

	// Timeout for API calls in seconds
	Timeout int

	// MaxRetries for failed API calls
	MaxRetries int

	// RateLimitPerMinute limits how many push notifications per minute
	RateLimitPerMinute int
}

// Provider implements the notification.Notifier interface for Duo Push
type Provider struct {
	config      *Config
	authAPI     *authapi.AuthApi
	rateLimiter *RateLimiter
	configured  bool
	mu          sync.RWMutex
}

// RateLimiter implements a simple rate limiter
type RateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	interval := time.Minute / time.Duration(requestsPerMinute)
	rl := &RateLimiter{
		tokens:   make(chan struct{}, requestsPerMinute),
		interval: interval,
	}

	// Fill the bucket
	for i := 0; i < requestsPerMinute; i++ {
		rl.tokens <- struct{}{}
	}

	// Start token refill goroutine
	go rl.refill()

	return rl
}

// refill adds tokens at the specified interval
func (rl *RateLimiter) refill() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full, skip
		}
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// NewProvider creates a new Duo provider instance
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
	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &Config{}

	// Parse integration_key
	integrationKey, ok := config["integration_key"].(string)
	if !ok || integrationKey == "" {
		return ErrMissingIntegrationKey
	}
	cfg.IntegrationKey = integrationKey

	// Parse secret_key
	secretKey, ok := config["secret_key"].(string)
	if !ok || secretKey == "" {
		return ErrMissingSecretKey
	}
	cfg.SecretKey = secretKey

	// Parse api_hostname
	apiHostname, ok := config["api_hostname"].(string)
	if !ok || apiHostname == "" {
		return ErrMissingAPIHostname
	}
	cfg.APIHostname = apiHostname

	// Parse timeout with default
	cfg.Timeout = defaultTimeout
	if timeout, ok := config["timeout"].(int); ok && timeout > 0 {
		cfg.Timeout = timeout
	}

	// Parse max_retries with default
	cfg.MaxRetries = defaultMaxRetries
	if maxRetries, ok := config["max_retries"].(int); ok && maxRetries > 0 {
		cfg.MaxRetries = maxRetries
	}

	// Parse rate limit with default
	cfg.RateLimitPerMinute = defaultRateLimit
	if rateLimit, ok := config["rate_limit_per_minute"].(int); ok && rateLimit > 0 {
		cfg.RateLimitPerMinute = rateLimit
	}

	// Initialize Duo API client
	duoAPI := duoapi.NewDuoApi(
		cfg.IntegrationKey,
		cfg.SecretKey,
		cfg.APIHostname,
		"traverse-notification/1.0",
		duoapi.SetTimeout(time.Duration(cfg.Timeout)*time.Second),
	)

	// Create Auth API
	authAPI := authapi.NewAuthApi(*duoAPI)

	p.authAPI = authAPI
	p.config = cfg
	p.rateLimiter = NewRateLimiter(cfg.RateLimitPerMinute)
	p.configured = true

	return nil
}

// Notify sends a notification via Duo Push
func (p *Provider) Notify(ctx context.Context, notification notification.Notification) error {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return ErrNotConfigured
	}
	rateLimiter := p.rateLimiter
	authAPI := p.authAPI
	p.mu.RUnlock()

	// Check rate limit
	if !rateLimiter.Allow() {
		return ErrRateLimitExceeded
	}

	// Validate notification type - Duo only supports approval requests
	switch notification.Type {
	case notification.TypeApprovalRequest:
		return p.sendPushNotification(ctx, authAPI, notification)
	default:
		return ErrUnsupportedType
	}
}

// sendPushNotification sends a Duo Push notification for approval
func (p *Provider) sendPushNotification(ctx context.Context, authAPI *authapi.AuthApi, notif notification.Notification) error {
	// Prepare push options
	username := notif.Recipient

	// Call Duo Preauth to check if user can receive push
	preauthOptions := []func(*url.Values){
		authapi.PreauthUsername(username),
	}

	// Call Duo Auth API with retry logic
	var result *authapi.PreauthResult
	var err error

	operation := func() error {
		result, err = authAPI.Preauth(preauthOptions...)
		if err != nil {
			return err
		}
		return nil
	}

	if err := withRetry(operation, p.config.MaxRetries, 1*time.Second); err != nil {
		return fmt.Errorf("%w: %v", ErrDuoAPIError, err)
	}

	// Check if user can receive push
	if result.Response.Result != "auth" {
		return fmt.Errorf("%w: user cannot receive push notifications: %s", ErrDuoAPIError, result.Response.Status_Msg)
	}

	return nil
}

// Close cleans up any resources used by the provider
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.authAPI != nil {
		p.authAPI = nil
	}

	if p.rateLimiter != nil {
		// Rate limiter goroutine will stop when tokens channel is garbage collected
		p.rateLimiter = nil
	}

	p.configured = false
	return nil
}

// withRetry executes an operation with retry logic
func withRetry(operation func() error, maxRetries int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if err := operation(); err != nil {
			lastErr = err
			if i < maxRetries-1 {
				time.Sleep(delay)
			}
		} else {
			return nil
		}
	}

	return lastErr
}

// DuoResponse represents a response from the Duo API
type DuoResponse struct {
	Stat     string `json:"stat"`
	Response struct {
		Result    string `json:"result"`
		Status    string `json:"status"`
		StatusMsg string `json:"status_msg"`
	} `json:"response"`
}
