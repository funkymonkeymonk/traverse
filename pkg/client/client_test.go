package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid client creation",
			baseURL: "http://localhost:8080",
			apiKey:  "test-api-key",
			wantErr: false,
		},
		{
			name:    "client without API key",
			baseURL: "http://localhost:8080",
			apiKey:  "",
			wantErr: false,
		},
		{
			name:    "client with trailing slash in URL",
			baseURL: "http://localhost:8080/",
			apiKey:  "test-key",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL, tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestCreateRequest(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/requests" {
			t.Errorf("Expected path /v1/requests, got %s", r.URL.Path)
		}

		var req CreateRequestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		response := CreateRequestResponse{
			RequestID:             "req_12345",
			Status:                "pending_approval",
			Message:               "Request submitted",
			PollURL:               "/v1/requests/req_12345/status",
			WebSocketURL:          "wss://example.com/v1/requests/req_12345/stream",
			ExpiresAt:             time.Now().Add(5 * time.Minute),
			EstimatedApprovalTime: "< 2 minutes",
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := CreateRequestRequest{
		SecretPath: "prod/api/key",
		Reason:     "Need access for deployment",
		ClientID:   "test-client",
	}

	resp, err := client.CreateRequest(req)
	if err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}

	if resp.RequestID != "req_12345" {
		t.Errorf("Expected request ID req_12345, got %s", resp.RequestID)
	}
	if resp.Status != "pending_approval" {
		t.Errorf("Expected status pending_approval, got %s", resp.Status)
	}
}

func TestGetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/requests/req_12345/status" {
			t.Errorf("Expected path /v1/requests/req_12345/status, got %s", r.URL.Path)
		}

		response := GetStatusResponse{
			RequestID:  "req_12345",
			Status:     "pending",
			ClientID:   "test-client",
			SecretPath: "prod/api/key",
			Reason:     "Need access",
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(5 * time.Minute),
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	resp, err := client.GetStatus("req_12345")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if resp.RequestID != "req_12345" {
		t.Errorf("Expected request ID req_12345, got %s", resp.RequestID)
	}
	if resp.Status != "pending" {
		t.Errorf("Expected status pending, got %s", resp.Status)
	}
}

func TestApproveRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/requests/req_12345/approve" {
			t.Errorf("Expected path /v1/requests/req_12345/approve, got %s", r.URL.Path)
		}

		response := ApproveRequestResponse{
			RequestID:                  "req_12345",
			Status:                     "approved",
			Message:                    "Request approved",
			ApprovedAt:                 time.Now(),
			RemainingRequiredApprovals: 0,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := ApproveRequestRequest{
		Reason: "Approved by admin",
	}

	resp, err := client.ApproveRequest("req_12345", req)
	if err != nil {
		t.Fatalf("ApproveRequest() error = %v", err)
	}

	if resp.RequestID != "req_12345" {
		t.Errorf("Expected request ID req_12345, got %s", resp.RequestID)
	}
	if resp.Status != "approved" {
		t.Errorf("Expected status approved, got %s", resp.Status)
	}
}

func TestDenyRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/requests/req_12345/deny" {
			t.Errorf("Expected path /v1/requests/req_12345/deny, got %s", r.URL.Path)
		}

		response := DenyRequestResponse{
			RequestID: "req_12345",
			Status:    "denied",
			Message:   "Request denied",
			DeniedAt:  time.Now(),
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := DenyRequestRequest{
		Reason: "Access not authorized",
	}

	resp, err := client.DenyRequest("req_12345", req)
	if err != nil {
		t.Fatalf("DenyRequest() error = %v", err)
	}

	if resp.RequestID != "req_12345" {
		t.Errorf("Expected request ID req_12345, got %s", resp.RequestID)
	}
	if resp.Status != "denied" {
		t.Errorf("Expected status denied, got %s", resp.Status)
	}
}

func TestGetSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/secrets/prod/api/key" {
			t.Errorf("Expected path /v1/secrets/prod/api/key, got %s", r.URL.Path)
		}

		token := r.URL.Query().Get("token")
		if token != "valid-token" {
			t.Errorf("Expected token valid-token, got %s", token)
		}

		response := SecretResponse{
			Path:     "prod/api/key",
			Provider: "1password",
			Values: map[string]string{
				"api_key": "secret-value",
			},
			Access: AccessInfo{
				GrantedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
				RequestID: "req_12345",
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	resp, err := client.GetSecret("prod/api/key", "valid-token")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}

	if resp.Path != "prod/api/key" {
		t.Errorf("Expected path prod/api/key, got %s", resp.Path)
	}
	if resp.Values["api_key"] != "secret-value" {
		t.Errorf("Expected api_key secret-value, got %s", resp.Values["api_key"])
	}
}

func TestClientWithInvalidServer(t *testing.T) {
	client, err := NewClient("http://invalid-server:99999", "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetStatus("req_12345")
	if err == nil {
		t.Error("Expected error for invalid server, got nil")
	}
}

func TestClientHandlesErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":   "https://traverse.internal/errors/not-found",
			"title":  "Not Found",
			"status": 404,
			"detail": "Request not found",
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetStatus("req_invalid")
	if err == nil {
		t.Error("Expected error for 404 response, got nil")
	}
}

func TestPollStatus(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		status := "pending"
		if callCount >= 3 {
			status = "approved"
		}

		response := GetStatusResponse{
			RequestID:  "req_12345",
			Status:     status,
			ClientID:   "test-client",
			SecretPath: "prod/api/key",
			Reason:     "Need access",
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(5 * time.Minute),
		}

		if status == "approved" {
			now := time.Now()
			response.ApprovedAt = &now
			response.Token = "access-token"
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Poll with a short interval for testing
	resp, err := client.PollStatus("req_12345", 100*time.Millisecond, 5)
	if err != nil {
		t.Fatalf("PollStatus() error = %v", err)
	}

	if resp.Status != "approved" {
		t.Errorf("Expected status approved, got %s", resp.Status)
	}
	if resp.Token != "access-token" {
		t.Errorf("Expected token access-token, got %s", resp.Token)
	}
	if callCount < 3 {
		t.Errorf("Expected at least 3 calls, got %d", callCount)
	}
}

func TestPollStatusTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetStatusResponse{
			RequestID:  "req_12345",
			Status:     "pending",
			ClientID:   "test-client",
			SecretPath: "prod/api/key",
			Reason:     "Need access",
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(5 * time.Minute),
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Poll with max 2 attempts
	_, err = client.PollStatus("req_12345", 100*time.Millisecond, 2)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
