// Package onepassword implements a provider for 1Password Connect
package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/provider"
)

const (
	providerName       = "1password"
	defaultTimeout     = 30
	defaultMaxRetries  = 3
	defaultCacheTTL    = 5 * time.Minute
)

// Common errors
var (
	ErrNotConfigured = errors.New("provider not configured")
)

// Provider implements the provider.Provider interface for 1Password Connect
type Provider struct {
	config     *Config
	client     *http.Client
	cache      *SecretCache
	configured bool
	mu         sync.RWMutex
}

// Secret represents a cached secret entry
type Secret struct {
	Path       string
	Values     map[string]string
	Metadata   provider.SecretMetadata
	CachedAt   time.Time
}

// NewProvider creates a new 1Password provider instance
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
	
	// Parse host
	host, ok := config["host"].(string)
	if !ok || host == "" {
		return fmt.Errorf("host is required")
	}
	cfg.Host = host

	// Parse token
	token, ok := config["token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("token is required")
	}
	cfg.Token = token

	// Parse vault (optional)
	if vault, ok := config["vault"].(string); ok {
		cfg.Vault = vault
	}

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

	// Initialize HTTP client
	p.client = &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	// Initialize cache
	p.cache = NewSecretCache(defaultCacheTTL)

	p.config = cfg
	p.configured = true

	return nil
}

// Get retrieves a secret by its path
func (p *Provider) Get(ctx context.Context, path string) (*provider.Secret, error) {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return nil, ErrNotConfigured
	}
	p.mu.RUnlock()

	// Validate path
	if err := validatePath(path); err != nil {
		return nil, err
	}

	// Normalize path
	path = normalizePath(path)

	// Check cache first
	if cached, found := p.cache.Get(path); found {
		return &provider.Secret{
			Path:     cached.Path,
			Values:   cached.Values,
			Metadata: cached.Metadata,
		}, nil
	}

	// Fetch from 1Password Connect
	secret, err := p.fetchFromConnect(ctx, path)
	if err != nil {
		return nil, err
	}

	// Cache the result
	p.cache.Set(path, &Secret{
		Path:     secret.Path,
		Values:   secret.Values,
		Metadata: secret.Metadata,
		CachedAt: time.Now(),
	})

	return secret, nil
}

// List returns available secret paths under the given prefix
// Note: 1Password Connect API doesn't support listing all secrets,
// so this returns an empty list
func (p *Provider) List(ctx context.Context, prefix string) ([]string, error) {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return nil, ErrNotConfigured
	}
	p.mu.RUnlock()

	// 1Password Connect doesn't support listing all items
	// Returns empty list as specified in interface
	return []string{}, nil
}

// Health checks the provider's connectivity and health
func (p *Provider) Health(ctx context.Context) error {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return ErrNotConfigured
	}
	config := p.config
	p.mu.RUnlock()

	// Try to hit the health endpoint
	url := fmt.Sprintf("%s/health", config.Host)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Token))

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources used by the provider
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache != nil {
		p.cache.Clear()
		p.cache = nil
	}

	if p.client != nil {
		p.client.CloseIdleConnections()
	}

	return nil
}

// fetchFromConnect retrieves a secret from 1Password Connect API
func (p *Provider) fetchFromConnect(ctx context.Context, path string) (*provider.Secret, error) {
	config := p.config

	// Parse path to extract vault and item
	vault, item, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	// If no vault in path, use configured vault
	if vault == "" {
		vault = config.Vault
	}

	if vault == "" {
		return nil, fmt.Errorf("vault not specified in path or config")
	}

	var result *provider.Secret
	operation := func() error {
		url := fmt.Sprintf("%s/v1/vaults/%s/items/%s", config.Host, vault, item)
		
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Token))
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return provider.ErrSecretNotFound
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("1Password API returned status %d: %s", resp.StatusCode, string(body))
		}

		var itemResponse struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Fields []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"fields"`
			Version int       `json:"version"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&itemResponse); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		values := make(map[string]string)
		for _, field := range itemResponse.Fields {
			if field.Label != "" && field.Value != "" {
				values[field.Label] = field.Value
			}
		}

		result = &provider.Secret{
			Path:   path,
			Values: values,
			Metadata: provider.SecretMetadata{
				Version:   fmt.Sprintf("%d", itemResponse.Version),
				CreatedAt: itemResponse.CreatedAt,
				UpdatedAt: itemResponse.UpdatedAt,
			},
		}

		return nil
	}

	if err := withRetry(operation, config.MaxRetries, 1*time.Second); err != nil {
		return nil, err
	}

	return result, nil
}

// validatePath validates a secret path
func validatePath(path string) error {
	if path == "" {
		return provider.ErrInvalidPath
	}
	if strings.Contains(path, "..") {
		return provider.ErrInvalidPath
	}
	return nil
}

// normalizePath normalizes a secret path to canonical form
func normalizePath(path string) string {
	// Remove leading slash if present
	path = strings.TrimPrefix(path, "/")
	// Remove trailing slash if present
	path = strings.TrimSuffix(path, "/")
	return path
}

// parsePath parses a path into vault and item components
// Expected format: vault/item or just item (uses default vault)
func parsePath(path string) (vault, item string, err error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", parts[0], nil
}

// withRetry executes an operation with retry logic
func withRetry(operation func() error, maxRetries int, delay time.Duration) error {
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if err := operation(); err != nil {
			lastErr = err
			if i < maxRetries-1 {
				time.Sleep(delay)
				continue
			}
		}
		return nil
	}
	
	return lastErr
}
