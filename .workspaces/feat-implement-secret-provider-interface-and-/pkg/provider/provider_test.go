package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockProvider is a test implementation of the Provider interface
type mockProvider struct {
	name           string
	configured     bool
	healthy        bool
	secrets        map[string]*Secret
	listPaths      []string
	configureError error
	getError       error
	listError      error
	healthError    error
	closeCalled    bool
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Configure(config map[string]interface{}) error {
	if m.configureError != nil {
		return m.configureError
	}
	m.configured = true
	return nil
}

func (m *mockProvider) Get(ctx context.Context, path string) (*Secret, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	secret, ok := m.secrets[path]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return secret, nil
}

func (m *mockProvider) List(ctx context.Context, prefix string) ([]string, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.listPaths, nil
}

func (m *mockProvider) Health(ctx context.Context) error {
	if m.healthError != nil {
		return m.healthError
	}
	if !m.healthy {
		return errors.New("provider is unhealthy")
	}
	return nil
}

func (m *mockProvider) Close() error {
	m.closeCalled = true
	return nil
}

func TestProviderInterface(t *testing.T) {
	t.Run("Provider implements all required methods", func(t *testing.T) {
		provider := &mockProvider{name: "test-provider"}

		// Test Name()
		if provider.Name() != "test-provider" {
			t.Errorf("expected name 'test-provider', got '%s'", provider.Name())
		}

		// Test Configure()
		err := provider.Configure(map[string]interface{}{"key": "value"})
		if err != nil {
			t.Errorf("unexpected error during configure: %v", err)
		}
		if !provider.configured {
			t.Error("expected provider to be configured")
		}

		// Test Get()
		ctx := context.Background()
		provider.secrets = map[string]*Secret{
			"test/path": {
				Path:   "test/path",
				Values: map[string]string{"key": "value"},
			},
		}
		secret, err := provider.Get(ctx, "test/path")
		if err != nil {
			t.Errorf("unexpected error during get: %v", err)
		}
		if secret == nil {
			t.Error("expected secret, got nil")
		}

		// Test List()
		provider.listPaths = []string{"path1", "path2"}
		paths, err := provider.List(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error during list: %v", err)
		}
		if len(paths) != 2 {
			t.Errorf("expected 2 paths, got %d", len(paths))
		}

		// Test Health()
		provider.healthy = true
		err = provider.Health(ctx)
		if err != nil {
			t.Errorf("unexpected error during health check: %v", err)
		}

		// Test Close()
		err = provider.Close()
		if err != nil {
			t.Errorf("unexpected error during close: %v", err)
		}
		if !provider.closeCalled {
			t.Error("expected Close() to be called")
		}
	})

	t.Run("Get returns error for non-existent secret", func(t *testing.T) {
		provider := &mockProvider{
			name:    "test-provider",
			secrets: map[string]*Secret{},
		}
		ctx := context.Background()
		_, err := provider.Get(ctx, "nonexistent/path")
		if !errors.Is(err, ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound, got %v", err)
		}
	})

	t.Run("Health returns error for unhealthy provider", func(t *testing.T) {
		provider := &mockProvider{
			name:    "test-provider",
			healthy: false,
		}
		ctx := context.Background()
		err := provider.Health(ctx)
		if err == nil {
			t.Error("expected error for unhealthy provider")
		}
	})
}

func TestSecretMetadata(t *testing.T) {
	t.Run("Secret contains expected metadata", func(t *testing.T) {
		now := time.Now()
		secret := &Secret{
			Path:   "test/path",
			Values: map[string]string{"api_key": "secret123"},
			Metadata: SecretMetadata{
				Version:   "1",
				CreatedAt: now,
				UpdatedAt: now,
				Tags:      map[string]string{"env": "prod"},
				Custom:    map[string]interface{}{"rotation": "daily"},
			},
		}

		if secret.Path != "test/path" {
			t.Errorf("expected path 'test/path', got '%s'", secret.Path)
		}
		if secret.Metadata.Version != "1" {
			t.Errorf("expected version '1', got '%s'", secret.Metadata.Version)
		}
		if secret.Metadata.Tags["env"] != "prod" {
			t.Errorf("expected tag env='prod', got '%s'", secret.Metadata.Tags["env"])
		}
	})
}

func TestRegistry(t *testing.T) {
	t.Run("Register adds provider factory", func(t *testing.T) {
		registry := NewRegistry()
		factory := func() Provider {
			return &mockProvider{name: "mock"}
		}

		registry.Register("mock", factory)

		providers := registry.List()
		found := false
		for _, name := range providers {
			if name == "mock" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'mock' to be in registered providers")
		}
	})

	t.Run("Create instantiates provider", func(t *testing.T) {
		registry := NewRegistry()
		factory := func() Provider {
			return &mockProvider{name: "mock-instance"}
		}

		registry.Register("mock", factory)
		provider, err := registry.Create("mock")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected provider, got nil")
		}
		if provider.Name() != "mock-instance" {
			t.Errorf("expected name 'mock-instance', got '%s'", provider.Name())
		}
	})

	t.Run("Create returns error for unknown provider", func(t *testing.T) {
		registry := NewRegistry()
		_, err := registry.Create("unknown")

		if !errors.Is(err, ErrProviderNotFound) {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})

	t.Run("List returns all registered providers", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register("provider1", func() Provider { return &mockProvider{name: "p1"} })
		registry.Register("provider2", func() Provider { return &mockProvider{name: "p2"} })
		registry.Register("provider3", func() Provider { return &mockProvider{name: "p3"} })

		providers := registry.List()
		if len(providers) != 3 {
			t.Errorf("expected 3 providers, got %d", len(providers))
		}
	})
}

func TestErrors(t *testing.T) {
	t.Run("Error constants are defined", func(t *testing.T) {
		if ErrProviderNotFound == nil {
			t.Error("ErrProviderNotFound should not be nil")
		}
		if ErrSecretNotFound == nil {
			t.Error("ErrSecretNotFound should not be nil")
		}
		if ErrAccessDenied == nil {
			t.Error("ErrAccessDenied should not be nil")
		}
		if ErrInvalidPath == nil {
			t.Error("ErrInvalidPath should not be nil")
		}
	})

	t.Run("Error messages are descriptive", func(t *testing.T) {
		if ErrProviderNotFound.Error() != "provider not found" {
			t.Errorf("unexpected error message: %s", ErrProviderNotFound.Error())
		}
		if ErrSecretNotFound.Error() != "secret not found" {
			t.Errorf("unexpected error message: %s", ErrSecretNotFound.Error())
		}
	})
}

func TestVersionedProvider(t *testing.T) {
	t.Run("VersionedProvider interface can be implemented", func(t *testing.T) {
		// This test verifies the VersionedProvider interface is properly defined
		// and can be implemented by concrete types
		var _ VersionedProvider = &mockVersionedProvider{}
	})
}

func TestWritableProvider(t *testing.T) {
	t.Run("WritableProvider interface can be implemented", func(t *testing.T) {
		// This test verifies the WritableProvider interface is properly defined
		// and can be implemented by concrete types
		var _ WritableProvider = &mockWritableProvider{}
	})
}

// mockVersionedProvider implements VersionedProvider for testing
type mockVersionedProvider struct {
	mockProvider
}

func (m *mockVersionedProvider) GetVersion(ctx context.Context, path, version string) (*Secret, error) {
	return &Secret{Path: path, Values: map[string]string{"version": version}}, nil
}

func (m *mockVersionedProvider) ListVersions(ctx context.Context, path string) ([]VersionInfo, error) {
	return []VersionInfo{
		{Version: "1", CreatedAt: time.Now(), IsCurrent: false},
		{Version: "2", CreatedAt: time.Now(), IsCurrent: true},
	}, nil
}

// mockWritableProvider implements WritableProvider for testing
type mockWritableProvider struct {
	mockProvider
}

func (m *mockWritableProvider) Set(ctx context.Context, path string, secret *Secret) error {
	return nil
}

func (m *mockWritableProvider) Delete(ctx context.Context, path string) error {
	return nil
}
