package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/funkymonkeymonk/traverse/internal/api"
	"github.com/funkymonkeymonk/traverse/internal/audit"
	"github.com/funkymonkeymonk/traverse/internal/auth"
	"github.com/funkymonkeymonk/traverse/internal/config"
	"github.com/funkymonkeymonk/traverse/internal/storage"
	"github.com/gin-gonic/gin"
)

type Server struct {
	httpServer *http.Server
	router     *gin.Engine
	storage    *storage.Database
	auditLog   audit.Logger
	config     *config.Config
}

type Config struct {
	Host      string
	Port      int
	CertFile  string
	KeyFile   string
	EnableTLS bool
}

func New(cfg *Config) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: router,
	}

	return &Server{
		httpServer: httpServer,
		router:     router,
	}, nil
}

func (s *Server) SetupRoutes(storage *storage.Database, auditLogger audit.Logger, cfg *config.Config) {
	s.storage = storage
	s.auditLog = auditLogger
	s.config = cfg

	handler := api.NewHandler(storage, auditLogger, cfg)

	// Build API keys map
	validKeys := make(map[string]auth.APIKey)
	for _, key := range cfg.Auth.APIKeys {
		validKeys[key.Key] = auth.APIKey{
			Key:          key.Key,
			ClientID:     key.ClientID,
			AllowedPaths: key.AllowedPaths,
		}
	}

	v1 := s.router.Group("/v1")
	{
		// Health check - no auth required
		v1.GET("/health", handler.HealthCheck)

		// Protected routes
		authRoutes := v1.Group("")
		authRoutes.Use(auth.APIKeyMiddleware(validKeys))
		{
			// Request management
			authRoutes.POST("/secrets/request", handler.CreateRequest)
			authRoutes.GET("/requests/:request_id/status", handler.GetRequestStatus)
			authRoutes.POST("/requests/:request_id/approve", handler.ApproveRequest)
			authRoutes.POST("/requests/:request_id/deny", handler.DenyRequest)
			authRoutes.GET("/requests", handler.ListRequests)

			// Token management
			authRoutes.POST("/tokens/:token_id/revoke", handler.RevokeToken)

			// Secret access with path authorization
			secretRoutes := authRoutes.Group("/secrets")
			secretRoutes.Use(auth.PathAuthorizationMiddleware())
			{
				secretRoutes.GET("/:path", handler.GetSecret)
			}
		}
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) StartTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Close database connection
	if s.storage != nil {
		if err := s.storage.Close(); err != nil {
			return fmt.Errorf("storage close error: %w", err)
		}
	}

	// Close audit logger
	if s.auditLog != nil {
		if err := s.auditLog.Close(); err != nil {
			return fmt.Errorf("audit log close error: %w", err)
		}
	}

	return nil
}
