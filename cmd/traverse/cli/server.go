package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/funkymonkeymonk/traverse/internal/audit"
	"github.com/funkymonkeymonk/traverse/internal/config"
	"github.com/funkymonkeymonk/traverse/internal/server"
	"github.com/funkymonkeymonk/traverse/internal/storage"
	"github.com/spf13/cobra"
)

var (
	serverConfigPath string
	serverHost       string
	serverPort       int
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Traverse server",
	Long: `Start the Traverse API server that handles secret requests,
approvals, and secret retrieval.`,
	Example: `  # Start server with default config
  traverse server

  # Start with custom config file
  traverse server --config /etc/traverse/config.yaml

  # Start on specific host and port
  traverse server --host 0.0.0.0 --port 9090`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// We need to override the persistent pre-run since server
		// needs to initialize itself differently
		return runServer()
	},
	// Override persistent pre-run for server command
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func runServer() error {
	// Load configuration
	cfg, err := config.Load(serverConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override with command line flags if provided
	if serverHost != "" {
		cfg.Server.Host = serverHost
	}
	if serverPort != 0 {
		cfg.Server.Port = serverPort
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
		return fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer db.Close()

	// Initialize audit logger
	auditConfig := map[string]string{
		"path": cfg.Audit.File.Path,
	}
	auditLogger, err := audit.NewLogger(cfg.Audit.Type, auditConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
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
		return fmt.Errorf("failed to create server: %w", err)
	}

	srv.SetupRoutes(db, auditLogger, cfg)

	// Log startup
	auditLogger.Log(audit.Event{
		Type:        "SERVER_START",
		Description: fmt.Sprintf("Traverse server starting on %s:%d", cfg.Server.Host, cfg.Server.Port),
	})

	fmt.Printf("Traverse v%s starting on %s:%d\n", Version, cfg.Server.Host, cfg.Server.Port)

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
		return fmt.Errorf("shutdown error: %w", err)
	}

	auditLogger.Log(audit.Event{
		Type:        "SERVER_STOP",
		Description: "Traverse server stopped gracefully",
	})

	fmt.Println("Server stopped")
	return nil
}

func init() {
	RootCmd.AddCommand(serverCmd)

	// Flags
	serverCmd.Flags().StringVarP(&serverConfigPath, "config", "c", "", "Path to configuration file")
	serverCmd.Flags().StringVar(&serverHost, "host", "", "Server host (overrides config)")
	serverCmd.Flags().IntVar(&serverPort, "port", 0, "Server port (overrides config)")
}
