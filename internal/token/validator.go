package token

import (
	"crypto/rsa"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Validator validates JWT tokens
type Validator struct {
	publicKey     *rsa.PublicKey
	issuer        string
	revokedTokens map[string]bool
	mu            sync.RWMutex
}

// NewValidator creates a new token validator with the given public key
func NewValidator(publicKey *rsa.PublicKey) *Validator {
	return &Validator{
		publicKey:     publicKey,
		issuer:        "traverse",
		revokedTokens: make(map[string]bool),
	}
}

// NewValidatorWithIssuer creates a validator with custom issuer
func NewValidatorWithIssuer(publicKey *rsa.PublicKey, issuer string) *Validator {
	return &Validator{
		publicKey:     publicKey,
		issuer:        issuer,
		revokedTokens: make(map[string]bool),
	}
}

// Validate parses and validates a JWT token
// Returns the claims if valid, or an error if invalid
func (v *Validator) Validate(tokenString string) (*TokenClaims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	}, jwt.WithIssuer(v.issuer))

	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	// Extract claims
	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Check if token has been revoked
	v.mu.RLock()
	revoked := v.revokedTokens[claims.ID]
	v.mu.RUnlock()

	if revoked {
		return nil, fmt.Errorf("token has been revoked")
	}

	return claims, nil
}

// ValidateForPath validates a token and checks if it grants access to a specific path
func (v *Validator) ValidateForPath(tokenString string, secretPath string) (*TokenClaims, error) {
	claims, err := v.Validate(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if the token's secret path matches the requested path
	// The token's path should be a prefix of the requested path
	if !pathMatches(claims.SecretPath, secretPath) {
		return nil, fmt.Errorf("token does not grant access to path: %s", secretPath)
	}

	return claims, nil
}

// pathMatches checks if the granted path matches or is a parent of the requested path
func pathMatches(grantedPath, requestedPath string) bool {
	// Exact match
	if grantedPath == requestedPath {
		return true
	}

	// Check if granted path is a prefix of requested path
	// This allows tokens for "prod" to access "prod/database/credentials"
	if len(requestedPath) > len(grantedPath) {
		prefix := grantedPath + "/"
		if len(requestedPath) >= len(prefix) && requestedPath[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// Revoke marks a token as revoked by its ID
func (v *Validator) Revoke(tokenString string) error {
	// Parse the token without validation to get the ID
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &TokenClaims{})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return fmt.Errorf("invalid claims type")
	}

	v.mu.Lock()
	v.revokedTokens[claims.ID] = true
	v.mu.Unlock()

	return nil
}

// RevokeByID marks a token ID as revoked
func (v *Validator) RevokeByID(tokenID string) {
	v.mu.Lock()
	v.revokedTokens[tokenID] = true
	v.mu.Unlock()
}

// IsRevoked checks if a token ID has been revoked
func (v *Validator) IsRevoked(tokenID string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.revokedTokens[tokenID]
}

// SetRevokedTokens sets the list of revoked token IDs (for testing)
func (v *Validator) SetRevokedTokens(tokenIDs []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.revokedTokens = make(map[string]bool)
	for _, id := range tokenIDs {
		v.revokedTokens[id] = true
	}
}

// GetTokenID extracts the token ID from a token string without full validation
func (v *Validator) GetTokenID(tokenString string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &TokenClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims type")
	}

	return claims.ID, nil
}

// GetRemainingTTL returns the remaining time until the token expires
// Returns an error if the token is invalid or expired
func (v *Validator) GetRemainingTTL(tokenString string) (time.Duration, error) {
	claims, err := v.Validate(tokenString)
	if err != nil {
		return 0, err
	}

	if claims.ExpiresAt == nil {
		return 0, fmt.Errorf("token has no expiration")
	}

	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		return 0, fmt.Errorf("token has expired")
	}

	return remaining, nil
}

// GetPublicKey returns the validator's public key
func (v *Validator) GetPublicKey() *rsa.PublicKey {
	return v.publicKey
}

// GetIssuer returns the expected issuer
func (v *Validator) GetIssuer() string {
	return v.issuer
}

// Claims represents the parsed JWT claims
// This is an alias for TokenClaims for backward compatibility
type Claims = TokenClaims
