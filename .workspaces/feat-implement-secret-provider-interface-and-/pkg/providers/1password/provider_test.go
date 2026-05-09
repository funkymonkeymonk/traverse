package onepassword

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with default config", func(t *testing.T) {
		provider := NewProvider()
		if provider == nil {
			t.Fatal("expected provider, got nil")
		}
		if provider.Name() != "1password" {
			t.Errorf("expected name '1password', got '%s'", provider.Name())
		}
	})
}

func TestProvider_Configure(t *testing.T) {
	t.Run("configures with valid config", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"host":        "http://localhost:8080",
			"token":       "test-token",
			"vault":       "test-vault",
			"timeout":     30,
			"max_retries": 3,
		}

		err := provider.Configure(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.config.Host != "http://localhost:8080" {
			t.Errorf("expected host 'http://localhost:8080', got '%s'", provider.config.Host)
		}
		if provider.config.Token != "test-token" {
			t.Errorf("expected token 'test-token', got '%s'", provider.config.Token)
		}
		if provider.config.Vault != "test-vault" {
			t.Errorf("expected vault 'test-vault', got '%s'", provider.config.Vault)
		}
	})

	t.Run("returns error for missing host", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"token": "test-token",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing host")
		}
	})

	t.Run("returns error for missing token", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"host": "http://localhost:8080",
		}

		err := provider.Configure(config)
		if err == nil {
			t.Error("expected error for missing token")
		}
	})

	t.Run("applies default timeout", func(t *testing.T) {
		provider := NewProvider()
		config := map[string]interface{}{
			"host":  "http://localhost:8080",
			"token": "test-token",
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
			"host":  "http://localhost:8080",
			"token": "test-token",
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

func TestProvider_Get(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		provider := NewProvider()
		ctx := context.Background()

		_, err := provider.Get(ctx, "test/path")
		if !errors.Is(err, ErrNotConfigured) {
			t.Errorf("expected ErrNotConfigured, got %v", err)
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		provider := createConfiguredProvider(t)
		ctx := context.Background()

		_, err := provider.Get(ctx, "")
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})

	t.Run("returns secret from cache if available", func(t *testing.T) {
		provider := createConfiguredProvider(t)
		ctx := context.Background()

		// Pre-populate cache
		expectedSecret := &Secret{
			Path:   "test/path",
			Values: map[string]string{"key": "cached-value"},
		}
		provider.cache.Set("test/path", expectedSecret)

		secret, err := provider.Get(ctx, "test/path")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if secret.Values["key"] != "cached-value" {
			t.Errorf("expected cached value, got '%s'", secret.Values["key"])
		}
	})
}

func TestProvider_List(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		provider := NewProvider()
		ctx := context.Background()

		_, err := provider.List(ctx, "test")
		if !errors.Is(err, ErrNotConfigured) {
			t.Errorf("expected ErrNotConfigured, got %v", err)
		}
	})

	t.Run("returns empty list when 1Password doesn't support listing", func(t *testing.T) {
		provider := createConfiguredProvider(t)
		ctx := context.Background()

		// 1Password Connect doesn't support listing all secrets
		// so we return empty list
		paths, err := provider.List(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("expected empty list, got %v", paths)
		}
	})
}

func TestProvider_Health(t *testing.T) {
	t.Run("returns error when not configured", func(t *testing.T) {
		provider := NewProvider()
		ctx := context.Background()

		err := provider.Health(ctx)
		if !errors.Is(err, ErrNotConfigured) {
			t.Errorf("expected ErrNotConfigured, got %v", err)
		}
	})

	t.Run("returns error for unreachable host", func(t *testing.T) {
		provider := createConfiguredProvider(t)
		provider.config.Host = "http://invalid-host:99999"
		ctx := context.Background()

		err := provider.Health(ctx)
		if err == nil {
			t.Error("expected error for unreachable host")
		}
	})
}

func TestProvider_Close(t *testing.T) {
	t.Run("closes provider and cleans up resources", func(t *testing.T) {
		provider := createConfiguredProvider(t)

		err := provider.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if provider.cache != nil {
			t.Error("expected cache to be nil after close")
		}
	})

	t.Run("does not error when called multiple times", func(t *testing.T) {
		provider := createConfiguredProvider(t)

		err := provider.Close()
		if err != nil {
			t.Errorf("unexpected error on first close: %v", err)
		}

		err = provider.Close()
		if err != nil {
			t.Errorf("unexpected error on second close: %v", err)
		}
	})
}

func TestSecretCache(t *testing.T) {
	t.Run("stores and retrieves secrets", func(t *testing.T) {
		cache := NewSecretCache(5 * time.Minute)
		secret := &Secret{
			Path:   "test/path",
			Values: map[string]string{"key": "value"},
		}

		cache.Set("test/path", secret)
		retrieved, found := cache.Get("test/path")

		if !found {
			t.Error("expected to find secret in cache")
		}
		if retrieved.Values["key"] != "value" {
			t.Errorf("expected value 'value', got '%s'", retrieved.Values["key"])
		}
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		cache := NewSecretCache(5 * time.Minute)

		_, found := cache.Get("nonexistent/path")
		if found {
			t.Error("expected not to find secret in cache")
		}
	})

	t.Run("deletes secrets", func(t *testing.T) {
		cache := NewSecretCache(5 * time.Minute)
		secret := &Secret{Path: "test/path", Values: map[string]string{}}

		cache.Set("test/path", secret)
		cache.Delete("test/path")

		_, found := cache.Get("test/path")
		if found {
			t.Error("expected secret to be deleted from cache")
		}
	})

	t.Run("clears all secrets", func(t *testing.T) {
		cache := NewSecretCache(5 * time.Minute)
		cache.Set("path1", &Secret{Path: "path1", Values: map[string]string{}})
		cache.Set("path2", &Secret{Path: "path2", Values: map[string]string{}})

		cache.Clear()

		_, found1 := cache.Get("path1")
		_, found2 := cache.Get("path2")
		if found1 || found2 {
			t.Error("expected all secrets to be cleared from cache")
		}
	})

	t.Run("expires entries after TTL", func(t *testing.T) {
		cache := NewSecretCache(1 * time.Millisecond)
		secret := &Secret{Path: "test/path", Values: map[string]string{"key": "value"}}

		cache.Set("test/path", secret)
		time.Sleep(10 * time.Millisecond) // Wait for expiration

		_, found := cache.Get("test/path")
		if found {
			t.Error("expected secret to be expired from cache")
		}
	})
}

func TestRetryLogic(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		err := withRetry(operation, 3, 1*time.Millisecond)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries on failure and eventually succeeds", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}

		err := withRetry(operation, 5, 1*time.Millisecond)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("fails after max retries exceeded", func(t *testing.T) {
		attempts := 0
		expectedErr := errors.New("persistent error")
		operation := func() error {
			attempts++
			return expectedErr
		}

		err := withRetry(operation, 3, 1*time.Millisecond)
		if err == nil {
			t.Error("expected error after max retries")
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})
}

// Helper function to create a configured provider
func createConfiguredProvider(t *testing.T) *Provider {
	t.Helper()
	provider := NewProvider()
	config := map[string]interface{}{
		"host":        "http://localhost:8080",
		"token":       "test-token",
		"vault":       "test-vault",
		"timeout":     30,
		"max_retries": 3,
	}

	err := provider.Configure(config)
	if err != nil {
		t.Fatalf("failed to configure provider: %v", err)
	}

	return provider
}
