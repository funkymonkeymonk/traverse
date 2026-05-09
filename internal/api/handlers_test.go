package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/funkymonkeymonk/traverse/internal/audit"
	"github.com/funkymonkeymonk/traverse/internal/auth"
	"github.com/funkymonkeymonk/traverse/internal/config"
	"github.com/funkymonkeymonk/traverse/internal/storage"
	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *MockStorage, *MockAuditLogger) {
	gin.SetMode(gin.TestMode)

	mockStorage := &MockStorage{}
	auditLogger := &MockAuditLogger{}
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080},
		Auth: config.AuthConfig{
			Type: "api_key",
			APIKeys: []config.APIKeyConfig{
				{Key: "test_key", ClientID: "agent-001", AllowedPaths: []string{"*"}},
			},
		},
	}

	handler := NewHandler(mockStorage, auditLogger, cfg)

	// Build API keys map for auth middleware
	validKeys := make(map[string]auth.APIKey)
	for _, key := range cfg.Auth.APIKeys {
		validKeys[key.Key] = auth.APIKey{
			Key:          key.Key,
			ClientID:     key.ClientID,
			AllowedPaths: key.AllowedPaths,
		}
	}

	router := gin.New()
	v1 := router.Group("/v1")
	{
		// Health check - no auth required
		v1.GET("/health", handler.HealthCheck)

		// Protected routes
		authRoutes := v1.Group("")
		authRoutes.Use(auth.APIKeyMiddleware(validKeys))
		{
			authRoutes.POST("/secrets/request", handler.CreateRequest)
			authRoutes.GET("/requests/:request_id/status", handler.GetRequestStatus)
			authRoutes.POST("/requests/:request_id/approve", handler.ApproveRequest)
			authRoutes.POST("/requests/:request_id/deny", handler.DenyRequest)
			authRoutes.GET("/requests", handler.ListRequests)
			authRoutes.GET("/secrets/:path", handler.GetSecret)
			authRoutes.POST("/tokens/:token_id/revoke", handler.RevokeToken)
		}
	}

	return router, mockStorage, auditLogger
}

type MockStorage struct {
	requests map[string]*storage.SecretRequest
}

func (m *MockStorage) CreateRequest(req *storage.SecretRequest) error {
	if m.requests == nil {
		m.requests = make(map[string]*storage.SecretRequest)
	}
	m.requests[req.ID] = req
	return nil
}

func (m *MockStorage) GetRequest(id string) (*storage.SecretRequest, error) {
	if m.requests == nil {
		return nil, fmt.Errorf("request not found")
	}
	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("request not found")
	}
	return req, nil
}

func (m *MockStorage) UpdateRequestStatus(id string, status string) error {
	if m.requests != nil && m.requests[id] != nil {
		m.requests[id].Status = status
	}
	return nil
}

func (m *MockStorage) ListRequests(filters storage.ListFilters, limit int, offset int) ([]*storage.SecretRequest, int, error) {
	var result []*storage.SecretRequest
	for _, req := range m.requests {
		if filters.Status == "" || req.Status == filters.Status {
			result = append(result, req)
		}
	}
	return result, len(result), nil
}

type MockAuditLogger struct {
	events []audit.Event
}

func (m *MockAuditLogger) Log(event audit.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *MockAuditLogger) Close() error {
	return nil
}

func TestCreateRequest(t *testing.T) {
	router, mockStorage, auditLogger := setupTestRouter()

	body := map[string]interface{}{
		"secret_path": "prod/api-keys/stripe",
		"reason":      "Deploying payment feature",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/request", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("CreateRequest status = %v, want %v", w.Code, http.StatusAccepted)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "pending_approval" {
		t.Errorf("Response status = %v, want %v", response["status"], "pending_approval")
	}

	if len(auditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(auditLogger.events))
	}
	if auditLogger.events[0].Type != "REQUEST_CREATED" {
		t.Errorf("Audit event type = %v, want %v", auditLogger.events[0].Type, "REQUEST_CREATED")
	}

	if len(mockStorage.requests) != 1 {
		t.Errorf("Expected 1 request in storage, got %d", len(mockStorage.requests))
	}
}

func TestCreateRequestInvalidPath(t *testing.T) {
	router, _, _ := setupTestRouter()

	body := map[string]interface{}{
		"secret_path": "prod/api keys/stripe",
		"reason":      "Deploying payment feature",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/request", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreateRequest status with invalid path = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRequestMissingAuth(t *testing.T) {
	router, _, _ := setupTestRouter()

	body := map[string]interface{}{
		"secret_path": "prod/api-keys/stripe",
		"reason":      "Deploying payment feature",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/request", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("CreateRequest status without auth = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestGetRequestStatus(t *testing.T) {
	router, mockStorage, _ := setupTestRouter()

	// Add a mock request
	testReq := &storage.SecretRequest{
		ID:                "req_test_123",
		ClientID:          "agent-001",
		SecretPath:        "prod/api-keys/stripe",
		Reason:            "Deploying payment feature",
		Status:            "pending",
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
	}
	mockStorage.requests = map[string]*storage.SecretRequest{testReq.ID: testReq}

	req := httptest.NewRequest(http.MethodGet, "/v1/requests/req_test_123/status", nil)
	req.Header.Set("Authorization", "Bearer test_key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetRequestStatus status = %v, want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["request_id"] != "req_test_123" {
		t.Errorf("Response request_id = %v, want %v", response["request_id"], "req_test_123")
	}
	if response["status"] != "pending" {
		t.Errorf("Response status = %v, want %v", response["status"], "pending")
	}
}

func TestHealthCheck(t *testing.T) {
	router, _, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HealthCheck status = %v, want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Health status = %v, want %v", response["status"], "healthy")
	}
	if response["version"] == "" {
		t.Error("Health check missing version field")
	}
}

func TestListRequests(t *testing.T) {
	router, mockStorage, _ := setupTestRouter()

	// Add mock requests
	mockStorage.requests = map[string]*storage.SecretRequest{
		"req_1": {ID: "req_1", ClientID: "agent-001", SecretPath: "dev/secrets", Status: "pending"},
		"req_2": {ID: "req_2", ClientID: "agent-001", SecretPath: "prod/secrets", Status: "approved"},
		"req_3": {ID: "req_3", ClientID: "agent-002", SecretPath: "dev/secrets", Status: "pending"},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/requests", nil)
	req.Header.Set("Authorization", "Bearer test_key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListRequests status = %v, want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	requests, ok := response["requests"].([]interface{})
	if !ok {
		t.Fatalf("Expected requests array in response")
	}

	if len(requests) != 3 {
		t.Errorf("Expected 3 requests, got %d", len(requests))
	}

	pagination, ok := response["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected pagination object in response")
	}

	if pagination["total"] != float64(3) {
		t.Errorf("Expected total=3, got %v", pagination["total"])
	}
}
