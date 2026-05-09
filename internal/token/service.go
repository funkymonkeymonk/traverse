package token

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	RequestID  string        `json:"request_id"`
	ClientID   string        `json:"client_id"`
	SecretPath string        `json:"secret_path"`
	Duration   time.Duration `json:"duration"`
	jwt.RegisteredClaims
}

// Service provides JWT token generation and management
type Service struct {
	privateKey    *rsa.PrivateKey
	maxTTL        time.Duration
	issuer        string
	revokedTokens map[string]bool
}

// NewService creates a new token service with the given private key and max TTL
func NewService(privateKey *rsa.PrivateKey, maxTTL time.Duration) *Service {
	return &Service{
		privateKey:    privateKey,
		maxTTL:        maxTTL,
		issuer:        "traverse",
		revokedTokens: make(map[string]bool),
	}
}

// NewServiceWithIssuer creates a token service with custom issuer
func NewServiceWithIssuer(privateKey *rsa.PrivateKey, maxTTL time.Duration, issuer string) *Service {
	return &Service{
		privateKey:    privateKey,
		maxTTL:        maxTTL,
		issuer:        issuer,
		revokedTokens: make(map[string]bool),
	}
}

// Generate creates a new JWT token with the given claims
// The token duration is capped at the service's maxTTL
func (s *Service) Generate(claims TokenClaims) (string, error) {
	now := time.Now()

	// Calculate effective duration (capped at maxTTL)
	duration := claims.Duration
	if duration == 0 || duration > s.maxTTL {
		duration = s.maxTTL
	}

	// Create JWT claims
	jwtClaims := TokenClaims{
		RequestID:  claims.RequestID,
		ClientID:   claims.ClientID,
		SecretPath: claims.SecretPath,
		Duration:   duration,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.issuer,
			Subject:   claims.ClientID,
			ID:        generateTokenID(),
		},
	}

	// Create token with RS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)

	// Sign the token
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Revoke marks a token as revoked
// Note: This is a simple in-memory implementation
// In production, this should use a distributed cache or database
func (s *Service) Revoke(tokenString string) error {
	// Parse the token to get its ID
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &s.privateKey.PublicKey, nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse token for revocation: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if jti, ok := claims["jti"].(string); ok {
			s.revokedTokens[jti] = true
			return nil
		}
	}

	return fmt.Errorf("token does not have a valid ID")
}

// IsRevoked checks if a token ID has been revoked
func (s *Service) IsRevoked(tokenID string) bool {
	return s.revokedTokens[tokenID]
}

// GetMaxTTL returns the maximum allowed token TTL
func (s *Service) GetMaxTTL() time.Duration {
	return s.maxTTL
}

// GetIssuer returns the token issuer
func (s *Service) GetIssuer() string {
	return s.issuer
}

// generateTokenID generates a unique token ID
func generateTokenID() string {
	return fmt.Sprintf("jti_%d", time.Now().UnixNano())
}

// GenerateKeyPair generates a new RSA key pair for testing
// In production, keys should be loaded from secure storage
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}

// EncodePrivateKeyToPEM encodes an RSA private key to PEM format
func EncodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	return privateKeyPEM
}

// EncodePublicKeyToPEM encodes an RSA public key to PEM format
func EncodePublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	return publicKeyPEM, nil
}

// DecodePrivateKeyFromPEM decodes an RSA private key from PEM format
func DecodePrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// DecodePublicKeyFromPEM decodes an RSA public key from PEM format
func DecodePublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return publicKey, nil
}
