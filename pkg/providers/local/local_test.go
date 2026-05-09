package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/funkymonkeymonk/traverse/pkg/provider"
)

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with default config", func(t *testing.T) {
		p := NewProvider()
		if p == nil {
			t.Fatal("expected provider, got nil")
		}
		if p.Name() != "local" {
			t.Errorf("expected name 'local', got '%s'", p.Name())
		}
	})
}

func TestProvider_Configure(t *testing.T) {
	t.Run("configures with valid base_path", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewProvider()
		config := map[string]interface{}{
			"base_path": tempDir,
		}

		err := p.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error for missing base_path", func(t *testing.T) {
		p := NewProvider()
		config := map[string]interface{}{}

		err := p.Configure(config)
		if err == nil {
			t.Error("expected error for missing base_path")
		}
	})

	t.Run("returns error for non-existent base_path", func(t *testing.T) {
		p := NewProvider()
		config := map[string]interface{}{
			"base_path": "/nonexistent/path",
		}

		err := p.Configure(config)
		if err == nil {
			t.Error("expected error for non-existent base_path")
		}
	})

	t.Run("returns error when base_path is file", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "test")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		p := NewProvider()
		config := map[string]interface{}{
			"base_path": tempFile.Name(),
		}

		err = p.Configure(config)
		if err == nil {
			t.Error("expected error when base_path is file")
		}
	})
}

func TestProvider_Get(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		p := NewProvider()
		ctx := context.Background()

		_, err := p.Get(ctx, "test/path")
		if err == nil {
			t.Error("expected error when not configured")
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		p := createConfiguredProvider(t)
		ctx := context.Background()

		_, err := p.Get(ctx, "")
		if !errors.Is(err, provider.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})

	t.Run("returns error for path traversal attempt", func(t *testing.T) {
		p := createConfiguredProvider(t)
		ctx := context.Background()

		_, err := p.Get(ctx, "../etc/passwd")
		if !errors.Is(err, provider.ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})

	t.Run("returns secret from file", func(t *testing.T) {
		p, tempDir := createConfiguredProviderWithDir(t)
		ctx := context.Background()

		// Create a secret file
		secretData := `{"path":"test/secret","values":{"key":"value"}}`
		secretPath := filepath.Join(tempDir, "test", "secret.json")
		os.MkdirAll(filepath.Dir(secretPath), 0755)
		os.WriteFile(secretPath, []byte(secretData), 0644)

		secret, err := p.Get(ctx, "test/secret")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if secret == nil {
			t.Fatal("expected secret, got nil")
		}
		if secret.Values["key"] != "value" {
			t.Errorf("expected value 'value', got '%s'", secret.Values["key"])
		}
	})

	t.Run("returns error for non-existent secret", func(t *testing.T) {
		p := createConfiguredProvider(t)
		ctx := context.Background()

		_, err := p.Get(ctx, "nonexistent/path")
		if !errors.Is(err, provider.ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound, got %v", err)
		}
	})
}

func TestProvider_List(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		p := NewProvider()
		ctx := context.Background()

		_, err := p.List(ctx, "test")
		if err == nil {
			t.Error("expected error when not configured")
		}
	})

	t.Run("returns list of secrets", func(t *testing.T) {
		p, tempDir := createConfiguredProviderWithDir(t)
		ctx := context.Background()

		// Create some secret files
		os.MkdirAll(filepath.Join(tempDir, "dev"), 0755)
		os.WriteFile(filepath.Join(tempDir, "dev", "api-key.json"), []byte("{}"), 0644)
		os.WriteFile(filepath.Join(tempDir, "dev", "db-pass.json"), []byte("{}"), 0644)

		paths, err := p.List(ctx, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(paths) != 2 {
			t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
		}
	})

	t.Run("returns filtered list with prefix", func(t *testing.T) {
		p, tempDir := createConfiguredProviderWithDir(t)
		ctx := context.Background()

		// Create directory structure
		os.MkdirAll(filepath.Join(tempDir, "dev"), 0755)
		os.MkdirAll(filepath.Join(tempDir, "prod"), 0755)
		os.WriteFile(filepath.Join(tempDir, "dev", "api-key.json"), []byte("{}"), 0644)
		os.WriteFile(filepath.Join(tempDir, "prod", "api-key.json"), []byte("{}"), 0644)

		paths, err := p.List(ctx, "dev")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(paths) != 1 {
			t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
		}
		if paths[0] != "dev/api-key" {
			t.Errorf("expected 'dev/api-key', got '%s'", paths[0])
		}
	})
}

func TestProvider_Health(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		p := NewProvider()
		ctx := context.Background()

		err := p.Health(ctx)
		if err == nil {
			t.Error("expected error when not configured")
		}
	})

	t.Run("returns nil for healthy provider", func(t *testing.T) {
		p := createConfiguredProvider(t)
		ctx := context.Background()

		err := p.Health(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestProvider_Close(t *testing.T) {
	t.Run("closes without error", func(t *testing.T) {
		p := createConfiguredProvider(t)

		err := p.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", "test/path", false},
		{"empty path", "", true},
		{"path with traversal", "../test", true},
		{"path with double dots", "test/../other", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/test/path", "test/path"},
		{"test/path/", "test/path"},
		{"/test/path/", "test/path"},
		{"test/path", "test/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper functions
func createConfiguredProvider(t *testing.T) *Provider {
	t.Helper()
	tempDir := t.TempDir()
	p := NewProvider()
	config := map[string]interface{}{
		"base_path": tempDir,
	}

	err := p.Configure(config)
	if err != nil {
		t.Fatalf("failed to configure provider: %v", err)
	}

	return p
}

func createConfiguredProviderWithDir(t *testing.T) (*Provider, string) {
	t.Helper()
	tempDir := t.TempDir()
	p := NewProvider()
	config := map[string]interface{}{
		"base_path": tempDir,
	}

	err := p.Configure(config)
	if err != nil {
		t.Fatalf("failed to configure provider: %v", err)
	}

	return p, tempDir
}
