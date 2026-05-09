package token

import (
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token Service Tests
// These tests verify JWT generation with RS256 signing

func TestTokenService_Generate_Success(t *testing.T) {
	// Generate test keys
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/database/credentials",
		Duration:   30 * time.Minute,
	}

	token, err := service.Generate(claims)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token == "" {
		t.Error("Expected token to be generated")
	}

	// Verify the token can be parsed and validated
	validator := NewValidator(publicKey)
	parsedClaims, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Expected token to be valid, got error: %v", err)
	}

	if parsedClaims.RequestID != claims.RequestID {
		t.Errorf("Expected request ID %s, got: %s", claims.RequestID, parsedClaims.RequestID)
	}

	if parsedClaims.ClientID != claims.ClientID {
		t.Errorf("Expected client ID %s, got: %s", claims.ClientID, parsedClaims.ClientID)
	}
}

func TestTokenService_Generate_WithExpiry(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Create service with 1 hour default TTL
	service := NewService(privateKey, 1*time.Hour)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	// Validate and check expiry
	validator := NewValidator(publicKey)
	parsedClaims, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Token should expire in approximately 30 minutes (the requested duration)
	expectedExpiry := time.Now().Add(30 * time.Minute)
	timeDiff := parsedClaims.ExpiresAt.Time.Sub(expectedExpiry)
	if timeDiff < -1*time.Minute || timeDiff > 1*time.Minute {
		t.Errorf("Token expiry differs from expected by %v", timeDiff)
	}
}

func TestTokenService_Generate_WithLongDuration(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Service with 1 hour max TTL
	service := NewService(privateKey, 1*time.Hour)

	// Request 2 hours - should be capped at 1 hour
	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   2 * time.Hour,
	}

	token, _ := service.Generate(claims)

	validator := NewValidator(publicKey)
	parsedClaims, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Token should be capped at 1 hour max
	expectedExpiry := time.Now().Add(1 * time.Hour)
	timeDiff := parsedClaims.ExpiresAt.Time.Sub(expectedExpiry)
	if timeDiff < -1*time.Minute || timeDiff > 1*time.Minute {
		t.Errorf("Token expiry should be capped at 1 hour, differs by %v", timeDiff)
	}
}

func TestTokenService_RevokeToken(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	// Revoke the token
	err = service.Revoke(token)
	if err != nil {
		t.Fatalf("Expected no error on revoke, got: %v", err)
	}

	// Validate should now fail
	validator := NewValidator(publicKey)
	validator.SetRevokedTokens([]string{token})

	_, err = validator.Validate(token)
	if err == nil {
		t.Error("Expected error for revoked token")
	}
}

func TestTokenService_Generate_DifferentAlgorithms(t *testing.T) {
	privateKey, _, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, err := service.Generate(claims)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Parse without validation to check algorithm
	parser := jwt.NewParser()
	tokenObj, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("Expected to parse token, got error: %v", err)
	}

	if tokenObj.Method != jwt.SigningMethodRS256 {
		t.Errorf("Expected RS256 signing method, got: %v", tokenObj.Method)
	}
}

// Token Validator Tests

func TestTokenValidator_Validate_Success(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)
	validator := NewValidator(publicKey)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	parsedClaims, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if parsedClaims.RequestID != claims.RequestID {
		t.Errorf("Expected request ID %s, got: %s", claims.RequestID, parsedClaims.RequestID)
	}
}

func TestTokenValidator_Validate_InvalidSignature(t *testing.T) {
	// Generate two different key pairs
	_, publicKey1, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	privateKey2, _, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Sign with key2
	service := NewService(privateKey2, 1*time.Hour)
	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}
	token, _ := service.Generate(claims)

	// Validate with key1 - should fail
	validator := NewValidator(publicKey1)
	_, err = validator.Validate(token)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
}

func TestTokenValidator_Validate_Expired(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Create service with very short TTL
	service := NewService(privateKey, 1*time.Millisecond)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   1 * time.Millisecond,
	}

	token, _ := service.Generate(claims)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	validator := NewValidator(publicKey)
	_, err = validator.Validate(token)
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

func TestTokenValidator_Validate_InvalidToken(t *testing.T) {
	_, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	validator := NewValidator(publicKey)

	// Test various invalid tokens
	invalidTokens := []string{
		"not.a.token",
		"invalid",
		"",
	}

	for _, token := range invalidTokens {
		_, err := validator.Validate(token)
		if err == nil {
			t.Errorf("Expected error for invalid token: %s", token)
		}
	}
}

func TestTokenValidator_Validate_Revoked(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)
	validator := NewValidator(publicKey)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	// Revoke the token
	validator.Revoke(token)

	// Validation should fail
	_, err = validator.Validate(token)
	if err == nil {
		t.Error("Expected error for revoked token")
	}
}

func TestTokenValidator_GetTokenID(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)
	validator := NewValidator(publicKey)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	tokenID, err := validator.GetTokenID(token)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if tokenID == "" {
		t.Error("Expected non-empty token ID")
	}
}

func TestTokenValidator_GetRemainingTTL(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)
	validator := NewValidator(publicKey)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)

	ttl, err := validator.GetRemainingTTL(token)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// TTL should be approximately 30 minutes
	if ttl < 29*time.Minute || ttl > 30*time.Minute {
		t.Errorf("Expected TTL around 30 minutes, got: %v", ttl)
	}
}

func TestTokenValidator_GetRemainingTTL_Expired(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	// Create service with very short TTL
	service := NewService(privateKey, 1*time.Millisecond)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/secrets",
		Duration:   1 * time.Millisecond,
	}

	token, _ := service.Generate(claims)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	validator := NewValidator(publicKey)
	ttl, err := validator.GetRemainingTTL(token)
	if err == nil {
		t.Error("Expected error for expired token")
	}

	if ttl > 0 {
		t.Errorf("Expected zero or negative TTL for expired token, got: %v", ttl)
	}
}

func TestTokenService_ClaimsContainRequiredFields(t *testing.T) {
	privateKey, publicKey, err := generateTestKeys()
	if err != nil {
		t.Fatalf("Failed to generate test keys: %v", err)
	}

	service := NewService(privateKey, 1*time.Hour)
	validator := NewValidator(publicKey)

	claims := TokenClaims{
		RequestID:  "req_abc123",
		ClientID:   "client-1",
		SecretPath: "prod/database/credentials",
		Duration:   30 * time.Minute,
	}

	token, _ := service.Generate(claims)
	parsedClaims, _ := validator.Validate(token)

	// Verify all required fields are present
	if parsedClaims.RequestID == "" {
		t.Error("Expected request_id claim to be present")
	}
	if parsedClaims.ClientID == "" {
		t.Error("Expected client_id claim to be present")
	}
	if parsedClaims.SecretPath == "" {
		t.Error("Expected secret_path claim to be present")
	}
	if parsedClaims.IssuedAt == nil || parsedClaims.IssuedAt.IsZero() {
		t.Error("Expected issued_at claim to be present")
	}
	if parsedClaims.ExpiresAt == nil || parsedClaims.ExpiresAt.IsZero() {
		t.Error("Expected exp claim to be present")
	}
}

// Helper function to generate test RSA keys
func generateTestKeys() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	return GenerateKeyPair(2048)
}
