package notification

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager handles multi-channel notification delivery
type Manager struct {
	providers map[string]Notifier
	config    Config
	mu        sync.RWMutex
}

// Config holds manager configuration
type Config struct {
	// DefaultTimeout for notification delivery
	DefaultTimeout time.Duration

	// MaxRetries for failed notifications
	MaxRetries int

	// RetryBackoff is the delay between retries
	RetryBackoff time.Duration

	// ContinueOnError determines if delivery should continue to other providers on failure
	ContinueOnError bool
}

// NotificationResult tracks the outcome of a notification send operation
type NotificationResult struct {
	// NotificationID that was sent
	NotificationID string `json:"notification_id"`

	// SentAt is when the notification was sent
	SentAt time.Time `json:"sent_at"`

	// SuccessCount is how many providers successfully delivered
	SuccessCount int `json:"success_count"`

	// FailureCount is how many providers failed to deliver
	FailureCount int `json:"failure_count"`

	// Errors maps provider name to error message
	Errors map[string]string `json:"errors,omitempty"`
}

// AllSucceeded returns true if all providers successfully delivered
func (r NotificationResult) AllSucceeded() bool {
	return r.FailureCount == 0
}

// HasPartialSuccess returns true if at least one provider succeeded
func (r NotificationResult) HasPartialSuccess() bool {
	return r.SuccessCount > 0
}

// NewManager creates a new notification manager
func NewManager(config Config) *Manager {
	// Set defaults
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 5 * time.Second
	}

	return &Manager{
		providers: make(map[string]Notifier),
		config:    config,
	}
}

// RegisterProvider adds a notification provider to the manager
func (m *Manager) RegisterProvider(name string, notifier Notifier) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if notifier == nil {
		return fmt.Errorf("notifier cannot be nil")
	}

	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider '%s' is already registered", name)
	}

	m.providers[name] = notifier
	return nil
}

// UnregisterProvider removes a provider from the manager
func (m *Manager) UnregisterProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, exists := m.providers[name]
	if !exists {
		return ErrNotifierNotFound
	}

	// Close the provider
	if err := provider.Close(); err != nil {
		return fmt.Errorf("failed to close provider: %w", err)
	}

	delete(m.providers, name)
	return nil
}

// ConfigureProvider configures a specific provider
func (m *Manager) ConfigureProvider(name string, config map[string]interface{}) error {
	m.mu.RLock()
	provider, exists := m.providers[name]
	m.mu.RUnlock()

	if !exists {
		return ErrNotifierNotFound
	}

	return provider.Configure(config)
}

// Send sends a notification to all registered providers
func (m *Manager) Send(ctx context.Context, notification Notification) error {
	result := m.SendWithResult(ctx, notification)

	if result.FailureCount > 0 {
		return fmt.Errorf("notification delivery partially failed: %d/%d providers failed",
			result.FailureCount, result.SuccessCount+result.FailureCount)
	}

	return nil
}

// SendWithResult sends a notification and returns detailed results
func (m *Manager) SendWithResult(ctx context.Context, notification Notification) NotificationResult {
	result := NotificationResult{
		NotificationID: notification.ID,
		SentAt:         time.Now(),
		Errors:         make(map[string]string),
	}

	m.mu.RLock()
	providers := make(map[string]Notifier, len(m.providers))
	for name, provider := range m.providers {
		providers[name] = provider
	}
	m.mu.RUnlock()

	if len(providers) == 0 {
		result.Errors["manager"] = "no providers registered"
		result.FailureCount = 1
		return result
	}

	// Validate notification
	if err := notification.Validate(); err != nil {
		result.Errors["validation"] = err.Error()
		result.FailureCount = len(providers)
		return result
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, provider := range providers {
		wg.Add(1)
		go func(providerName string, p Notifier) {
			defer wg.Done()

			err := m.sendWithRetry(ctx, p, notification)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.FailureCount++
				result.Errors[providerName] = err.Error()
			} else {
				result.SuccessCount++
			}
		}(name, provider)
	}

	wg.Wait()
	return result
}

// SendWithPriority sends notifications only to providers that handle the given priority level
func (m *Manager) SendWithPriority(ctx context.Context, notification Notification, minPriority int) error {
	// For now, all providers receive all priority levels
	// This can be extended to filter providers based on their configured priority thresholds
	if notification.Priority < minPriority {
		return nil
	}

	return m.Send(ctx, notification)
}

// SendToProvider sends a notification to a specific provider
func (m *Manager) SendToProvider(ctx context.Context, providerName string, notification Notification) error {
	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		return ErrNotifierNotFound
	}

	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid notification: %w", err)
	}

	return m.sendWithRetry(ctx, provider, notification)
}

// sendWithRetry attempts to send a notification with retries
func (m *Manager) sendWithRetry(ctx context.Context, provider Notifier, notification Notification) error {
	var lastErr error

	for i := 0; i < m.config.MaxRetries; i++ {
		err := provider.Notify(ctx, notification)
		if err == nil {
			return nil
		}

		lastErr = err

		if i < m.config.MaxRetries-1 {
			select {
			case <-time.After(m.config.RetryBackoff):
				// Continue to next retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", m.config.MaxRetries, lastErr)
}

// ListProviders returns the names of all registered providers
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}

	return names
}

// GetProvider returns a provider by name
func (m *Manager) GetProvider(name string) (Notifier, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, ErrNotifierNotFound
	}

	return provider, nil
}

// GetProviderStatus returns the status of all providers
func (m *Manager) GetProviderStatus() map[string]ProviderStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]ProviderStatus, len(m.providers))
	for name := range m.providers {
		statuses[name] = ProviderStatus{
			Name:      name,
			Available: true,
		}
	}

	return statuses
}

// Close closes all registered providers
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for name, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close provider '%s': %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}

	return nil
}

// ProviderStatus represents the status of a notification provider
type ProviderStatus struct {
	// Name of the provider
	Name string `json:"name"`

	// Available indicates if the provider is available
	Available bool `json:"available"`

	// LastError is the last error encountered
	LastError string `json:"last_error,omitempty"`

	// LastUsed is when the provider was last used
	LastUsed time.Time `json:"last_used,omitempty"`
}
