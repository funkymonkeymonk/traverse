package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	cfg := &Config{
		Host: "127.0.0.1",
		Port: 0, // Let OS assign a port
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer server.Shutdown(context.Background())

	if server == nil {
		t.Error("New() returned nil server")
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	cfg := &Config{
		Host: "127.0.0.1",
		Port: 0,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Start() error = %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServer_Routes(t *testing.T) {
	cfg := &Config{
		Host: "127.0.0.1",
		Port: 0,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Verify router is initialized
	if server.router == nil {
		t.Error("Server router not initialized")
	}
}

func TestGracefulShutdownTimeout(t *testing.T) {
	cfg := &Config{
		Host: "127.0.0.1",
		Port: 0,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start server
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Start() error = %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Should complete without error (even if timeout is short)
	server.Shutdown(ctx)
}
