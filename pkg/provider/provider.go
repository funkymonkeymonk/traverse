// Package provider defines the interface for secret provider plugins
package provider

import (
	"context"
	"errors"
	"time"
)

// Provider is the interface that all secret providers must implement
type Provider interface {
	// Name returns the unique identifier for this provider
	Name() string

	// Configure initializes the provider with the given configuration
	// Called once during startup
	Configure(config map[string]interface{}) error

	// Get retrieves a secret by its path
	Get(ctx context.Context, path string) (*Secret, error)

	// List returns available secret paths under the given prefix
	// Returns empty slice if listing is not supported
	List(ctx context.Context, prefix string) ([]string, error)

	// Health checks the provider's connectivity and health
	Health(ctx context.Context) error

	// Close cleans up any resources used by the provider
	Close() error
}

// Secret represents a retrieved secret with its metadata
type Secret struct {
	// Path is the canonical path to the secret
	Path string `json:"path"`

	// Values contains the actual secret key-value pairs
	// Keys are strings, values are strings (e.g., "api_key": "sk_live_xxx")
	Values map[string]string `json:"values"`

	// Metadata contains provider-specific metadata
	Metadata SecretMetadata `json:"metadata"`
}

// SecretMetadata contains metadata about a secret
type SecretMetadata struct {
	// Version identifier (if versioned)
	Version string `json:"version,omitempty"`

	// CreatedAt is when the secret was created
	CreatedAt time.Time `json:"created_at,omitempty"`

	// UpdatedAt is when the secret was last modified
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// Tags are provider-specific labels
	Tags map[string]string `json:"tags,omitempty"`

	// Custom contains provider-specific fields
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// Common errors
var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrSecretNotFound   = errors.New("secret not found")
	ErrAccessDenied     = errors.New("access denied")
	ErrInvalidPath      = errors.New("invalid secret path")
)

// PathValidator validates secret paths
type PathValidator interface {
	// Validate checks if a path is valid for this provider
	Validate(path string) error

	// Normalize converts a path to canonical form
	Normalize(path string) string
}

// VersionedProvider is implemented by providers that support versioning
type VersionedProvider interface {
	Provider

	// GetVersion retrieves a specific version of a secret
	GetVersion(ctx context.Context, path, version string) (*Secret, error)

	// ListVersions returns available versions for a secret
	ListVersions(ctx context.Context, path string) ([]VersionInfo, error)
}

// VersionInfo contains information about a secret version
type VersionInfo struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	IsCurrent bool      `json:"is_current"`
}

// WritableProvider is implemented by providers that support writing secrets
type WritableProvider interface {
	Provider

	// Set creates or updates a secret
	Set(ctx context.Context, path string, secret *Secret) error

	// Delete removes a secret
	Delete(ctx context.Context, path string) error
}
