package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIKey struct {
	Key          string
	ClientID     string
	AllowedPaths []string
}

func APIKeyMiddleware(validKeys map[string]APIKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"type":   "https://traverse.internal/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Missing Authorization header",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"type":   "https://traverse.internal/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Invalid Authorization header format",
			})
			c.Abort()
			return
		}

		apiKey := parts[1]
		key, exists := validKeys[apiKey]
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"type":   "https://traverse.internal/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Set client context
		c.Set("client_id", key.ClientID)
		c.Set("allowed_paths", key.AllowedPaths)
		c.Next()
	}
}

func (k APIKey) IsPathAllowed(path string) bool {
	for _, pattern := range k.AllowedPaths {
		if pattern == "*" {
			return true
		}
		if pattern == path {
			return true
		}
		// Handle wildcard patterns like "dev/*"
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
	}
	return false
}

type MTLSConfig struct {
	CACertPath string
}

func MTLSMiddleware(config MTLSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, this would verify client certificates
		// For now, we'll assume the TLS termination handles mTLS verification
		// and we just extract the client identity from the certificate

		// This is a placeholder for mTLS support
		c.Next()
	}
}

func PathAuthorizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowedPaths, exists := c.Get("allowed_paths")
		if !exists {
			c.Next()
			return
		}

		paths, ok := allowedPaths.([]string)
		if !ok {
			c.Next()
			return
		}

		// Get the secret path from the request (for secret access endpoints)
		secretPath := c.Param("path")
		if secretPath == "" {
			c.Next()
			return
		}

		key := APIKey{AllowedPaths: paths}
		if !key.IsPathAllowed(secretPath) {
			c.JSON(http.StatusForbidden, gin.H{
				"type":   "https://traverse.internal/errors/access-denied",
				"title":  "Access Denied",
				"status": 403,
				"detail": "Client does not have access to this secret path",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
