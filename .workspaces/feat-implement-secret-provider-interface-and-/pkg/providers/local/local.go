// Package local implements a provider for local file-based secrets
package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/funkymonkeymonk/traverse/pkg/provider"
)

const providerName = "local"

// Provider implements the provider.Provider interface for local file-based secrets
type Provider struct {
	basePath   string
	configured bool
	mu         sync.RWMutex
}

// NewProvider creates a new local provider instance
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

	basePath, ok := config["base_path"].(string)
	if !ok || basePath == "" {
		return fmt.Errorf("base_path is required")
	}

	// Ensure the base path exists
	info, err := os.Stat(basePath)
	if err != nil {
		return fmt.Errorf("base_path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("base_path is not a directory")
	}

	p.basePath = basePath
	p.configured = true

	return nil
}

// Get retrieves a secret by its path
func (p *Provider) Get(ctx context.Context, path string) (*provider.Secret, error) {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return nil, errors.New("provider not configured")
	}
	p.mu.RUnlock()

	// Validate path
	if err := validatePath(path); err != nil {
		return nil, err
	}

	// Normalize path
	path = normalizePath(path)

	// Construct file path
	filePath := filepath.Join(p.basePath, path+".json")

	// Check for path traversal
	if !strings.HasPrefix(filePath, p.basePath) {
		return nil, provider.ErrInvalidPath
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, provider.ErrSecretNotFound
		}
		return nil, fmt.Errorf("failed to read secret file: %w", err)
	}

	// Parse JSON
	var secret provider.Secret
	if err := json.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret file: %w", err)
	}

	return &secret, nil
}

// List returns available secret paths under the given prefix
func (p *Provider) List(ctx context.Context, prefix string) ([]string, error) {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return nil, errors.New("provider not configured")
	}
	basePath := p.basePath
	p.mu.RUnlock()

	// Normalize prefix
	prefix = normalizePath(prefix)

	// Construct search path
	searchPath := basePath
	if prefix != "" {
		searchPath = filepath.Join(basePath, prefix)
	}

	var paths []string

	// Walk directory
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Convert file path back to secret path
			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				return nil
			}
			
			// Remove .json extension
			secretPath := strings.TrimSuffix(relPath, ".json")
			// Convert filepath separators to /
			secretPath = filepath.ToSlash(secretPath)
			
			paths = append(paths, secretPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	return paths, nil
}

// Health checks the provider's connectivity and health
func (p *Provider) Health(ctx context.Context) error {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return errors.New("provider not configured")
	}
	basePath := p.basePath
	p.mu.RUnlock()

	// Check if base path is accessible
	_, err := os.Stat(basePath)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// Close cleans up any resources used by the provider
func (p *Provider) Close() error {
	// No resources to clean up for local file provider
	return nil
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
