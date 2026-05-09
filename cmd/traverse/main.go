package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/funkymonkeymonk/traverse/internal/audit"
	"github.com/funkymonkeymonk/traverse/internal/config"
	"github.com/funkymonkeymonk/traverse/internal/server"
	"github.com/funkymonkeymonk/traverse/internal/storage"
)

var (
	version = "1.0.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Traverse v%s (commit: %s, date: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		fmt.Println("Traverse - MFA secrets proxy with approval workflows")
		fmt.Printf("Version: %s (commit: %s, date: %s)\n", version, commit, date)
		fmt.Println("")
		fmt.Println("Usage: traverse <command>")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  server    Start the Traverse server")
		fmt.Println("  version   Show version information")
		os.Exit(0)
	}

	switch os.Args[1] {
	case "server":
		runServer(*configPath)
	case "version":
		fmt.Printf("Traverse %s (commit: %s, date: %s)\n", version, commit, date)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServer(configPath string) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize storage
	var db *storage.Database
	switch cfg.Storage.Type {
	case "sqlite":
		db, err = storage.New("sqlite", cfg.Storage.SQLite.Path)
	case "postgres":
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Storage.Postgres.Host,
			cfg.Storage.Postgres.Port,
			cfg.Storage.Postgres.User,
			cfg.Storage.Postgres.Password,
			cfg.Storage.Postgres.Database,
			cfg.Storage.Postgres.SSLMode,
		)
		db, err = storage.New("postgres", connStr)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported storage type: %s\n", cfg.Storage.Type)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize audit logger
	auditConfig := map[string]string{
		"path": cfg.Audit.File.Path,
	}
	auditLogger, err := audit.NewLogger(cfg.Audit.Type, auditConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize audit logger: %v\n", err)
		os.Exit(1)
	}
	defer auditLogger.Close()

	// Create and configure server
	serverConfig := &server.Config{
		Host:      cfg.Server.Host,
		Port:      cfg.Server.Port,
		EnableTLS: cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != "",
		CertFile:  cfg.Server.TLS.CertFile,
		KeyFile:   cfg.Server.TLS.KeyFile,
	}

	srv, err := server.New(serverConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	srv.SetupRoutes(db, auditLogger, cfg)

	// Log startup
	auditLogger.Log(audit.Event{
		Type:        "SERVER_START",
		Description: fmt.Sprintf("Traverse server starting on %s:%d", cfg.Server.Host, cfg.Server.Port),
	})

	fmt.Printf("Traverse v%s starting on %s:%d\n", version, cfg.Server.Host, cfg.Server.Port)

	// Start server in a goroutine
	go func() {
		if serverConfig.EnableTLS {
			if err := srv.StartTLS(serverConfig.CertFile, serverConfig.KeyFile); err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			}
		} else {
			if err := srv.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down gracefully...")

	// Graceful shutdown
	ctx := context.Background()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		os.Exit(1)
	}

	auditLogger.Log(audit.Event{
		Type:        "SERVER_STOP",
		Description: "Traverse server stopped gracefully",
	})

	fmt.Println("Server stopped")
}
