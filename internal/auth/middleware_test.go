package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validKeys := map[string]APIKey{
		"traverse_api_valid123": {
			Key:          "traverse_api_valid123",
			ClientID:     "agent-001",
			AllowedPaths: []string{"dev/*", "staging/*"},
		},
	}

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantClient string
	}{
		{
			name:       "valid API key",
			authHeader: "Bearer traverse_api_valid123",
			wantStatus: http.StatusOK,
			wantClient: "agent-001",
		},
		{
			name:       "missing authorization header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantClient: "",
		},
		{
			name:       "invalid API key",
			authHeader: "Bearer traverse_api_invalid",
			wantStatus: http.StatusUnauthorized,
			wantClient: "",
		},
		{
			name:       "wrong authorization format",
			authHeader: "Basic traverse_api_valid123",
			wantStatus: http.StatusUnauthorized,
			wantClient: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(APIKeyMiddleware(validKeys))
			router.GET("/test", func(c *gin.Context) {
				clientID, _ := c.Get("client_id")
				c.JSON(http.StatusOK, gin.H{"client_id": clientID})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %v, want %v", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestAPIKeyAuthSetsClientContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validKeys := map[string]APIKey{
		"traverse_api_valid123": {
			Key:          "traverse_api_valid123",
			ClientID:     "agent-001",
			AllowedPaths: []string{"dev/*", "staging/*"},
		},
	}

	router := gin.New()
	router.Use(APIKeyMiddleware(validKeys))
	router.GET("/test", func(c *gin.Context) {
		clientID, exists := c.Get("client_id")
		if !exists {
			t.Error("client_id not set in context")
		}
		allowedPaths, exists := c.Get("allowed_paths")
		if !exists {
			t.Error("allowed_paths not set in context")
		}
		c.JSON(http.StatusOK, gin.H{
			"client_id":     clientID,
			"allowed_paths": allowedPaths,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer traverse_api_valid123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestPathAuthorization(t *testing.T) {
	tests := []struct {
		name         string
		allowedPaths []string
		path         string
		wantAllowed  bool
	}{
		{
			name:         "exact match",
			allowedPaths: []string{"dev/secrets"},
			path:         "dev/secrets",
			wantAllowed:  true,
		},
		{
			name:         "wildcard match",
			allowedPaths: []string{"dev/*"},
			path:         "dev/api-keys/stripe",
			wantAllowed:  true,
		},
		{
			name:         "no match",
			allowedPaths: []string{"dev/*"},
			path:         "prod/secrets",
			wantAllowed:  false,
		},
		{
			name:         "multiple patterns with match",
			allowedPaths: []string{"dev/*", "staging/*"},
			path:         "staging/secrets",
			wantAllowed:  true,
		},
		{
			name:         "empty allowed paths",
			allowedPaths: []string{},
			path:         "dev/secrets",
			wantAllowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := APIKey{
				Key:          "test",
				ClientID:     "test",
				AllowedPaths: tt.allowedPaths,
			}
			got := key.IsPathAllowed(tt.path)
			if got != tt.wantAllowed {
				t.Errorf("IsPathAllowed() = %v, want %v", got, tt.wantAllowed)
			}
		})
	}
}
