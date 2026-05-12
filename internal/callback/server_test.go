package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/notification"
)

// MockWorkflowEngine is a mock implementation for testing
type MockWorkflowEngine struct {
	approveCalled bool
	denyCalled    bool
	lastRequestID string
	lastIdentity  string
	lastReason    string
	approveResult *ApprovalResult
	denyResult    *DenialResult
	approveError  error
	denyError     error
}

func (m *MockWorkflowEngine) ApproveRequest(requestID string, identity string, reason string) (*ApprovalResult, error) {
	m.approveCalled = true
	m.lastRequestID = requestID
	m.lastIdentity = identity
	m.lastReason = reason
	if m.approveError != nil {
		return nil, m.approveError
	}
	if m.approveResult != nil {
		return m.approveResult, nil
	}
	return &ApprovalResult{
		RequestID: requestID,
		Status:    "approved",
	}, nil
}

func (m *MockWorkflowEngine) DenyRequest(requestID string, identity string, reason string) (*DenialResult, error) {
	m.denyCalled = true
	m.lastRequestID = requestID
	m.lastIdentity = identity
	m.lastReason = reason
	if m.denyError != nil {
		return nil, m.denyError
	}
	if m.denyResult != nil {
		return m.denyResult, nil
	}
	return &DenialResult{
		RequestID: requestID,
		Status:    "denied",
	}, nil
}

func TestNewServer(t *testing.T) {
	t.Run("creates server with default config", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		if server == nil {
			t.Fatal("expected server to be created")
		}
		if server.engine != engine {
			t.Error("expected engine to be set")
		}
		if server.config.Secret == "" {
			t.Error("expected default secret to be generated")
		}
	})

	t.Run("creates server with custom config", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		config := &Config{
			Secret: "custom-secret-key",
		}
		server := NewServer(engine, config)

		if server.config.Secret != "custom-secret-key" {
			t.Errorf("expected secret 'custom-secret-key', got '%s'", server.config.Secret)
		}
	})
}

func TestDuoCallback(t *testing.T) {
	t.Run("handles valid Duo approval callback", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			RequestID: "req-123",
			Response:  "approve",
			UserID:    "user@example.com",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", server.generateDuoSignature(body))

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		if !engine.approveCalled {
			t.Error("expected ApproveRequest to be called")
		}
		if engine.lastRequestID != "req-123" {
			t.Errorf("expected request_id 'req-123', got '%s'", engine.lastRequestID)
		}
		if engine.lastIdentity != "user@example.com" {
			t.Errorf("expected identity 'user@example.com', got '%s'", engine.lastIdentity)
		}
	})

	t.Run("handles valid Duo deny callback", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			RequestID: "req-456",
			Response:  "deny",
			UserID:    "user@example.com",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", server.generateDuoSignature(body))

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if !engine.denyCalled {
			t.Error("expected DenyRequest to be called")
		}
		if engine.lastRequestID != "req-456" {
			t.Errorf("expected request_id 'req-456', got '%s'", engine.lastRequestID)
		}
	})

	t.Run("rejects invalid Duo signature", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			RequestID: "req-123",
			Response:  "approve",
			UserID:    "user@example.com",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", "invalid-signature")

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}

		if engine.approveCalled {
			t.Error("expected ApproveRequest NOT to be called with invalid signature")
		}
	})

	t.Run("rejects missing request_id", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			Response:  "approve",
			UserID:    "user@example.com",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", server.generateDuoSignature(body))

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("rejects missing user_id", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			RequestID: "req-123",
			Response:  "approve",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", server.generateDuoSignature(body))

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("handles workflow engine approval error", func(t *testing.T) {
		engine := &MockWorkflowEngine{
			approveError: context.DeadlineExceeded,
		}
		server := NewServer(engine, nil)

		payload := DuoCallbackPayload{
			RequestID: "req-123",
			Response:  "approve",
			UserID:    "user@example.com",
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/duo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Duo-Signature", server.generateDuoSignature(body))

		rr := httptest.NewRecorder()
		server.HandleDuoCallback(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})
}

func TestSlackCallback(t *testing.T) {
	t.Run("handles valid Slack Block Kit approval", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, &Config{
			Secret:       "test-secret",
			SlackSecret:  "slack-signing-secret",
		})

		payload := SlackInteractionPayload{
			Type:      "block_actions",
			User:      SlackUser{ID: "U123456", Name: "testuser"},
			Actions:   []SlackAction{{ActionID: "approve_action", Value: "req-123"}},
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
		req.Header.Set("X-Slack-Signature", server.generateSlackSignature(body, timestamp))

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		if !engine.approveCalled {
			t.Error("expected ApproveRequest to be called")
		}
	})

	t.Run("handles valid Slack Block Kit denial", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, &Config{
			Secret:       "test-secret",
			SlackSecret:  "slack-signing-secret",
		})

		payload := SlackInteractionPayload{
			Type:      "block_actions",
			User:      SlackUser{ID: "U123456", Name: "testuser"},
			Actions:   []SlackAction{{ActionID: "deny_action", Value: "req-123"}},
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
		req.Header.Set("X-Slack-Signature", server.generateSlackSignature(body, timestamp))

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if !engine.denyCalled {
			t.Error("expected DenyRequest to be called")
		}
	})

	t.Run("rejects invalid Slack signature", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, &Config{
			Secret:       "test-secret",
			SlackSecret:  "slack-signing-secret",
		})

		payload := SlackInteractionPayload{
			Type:      "block_actions",
			User:      SlackUser{ID: "U123456", Name: "testuser"},
			Actions:   []SlackAction{{ActionID: "approve_action", Value: "req-123"}},
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Slack-Signature", "invalid-signature")

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}

		if engine.approveCalled {
			t.Error("expected ApproveRequest NOT to be called with invalid signature")
		}
	})

	t.Run("rejects stale Slack request", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, &Config{
			Secret:       "test-secret",
			SlackSecret:  "slack-signing-secret",
		})

		payload := SlackInteractionPayload{
			Type:      "block_actions",
			User:      SlackUser{ID: "U123456", Name: "testuser"},
			Actions:   []SlackAction{{ActionID: "approve_action", Value: "req-123"}},
			Timestamp: time.Now().Add(-10 * time.Minute).Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Slack-Request-Timestamp", string(rune(payload.Timestamp)))

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("ignores non-block_actions payloads", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := SlackInteractionPayload{
			Type:      "view_submission",
			User:      SlackUser{ID: "U123456", Name: "testuser"},
			Timestamp: time.Now().Unix(),
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if engine.approveCalled || engine.denyCalled {
			t.Error("expected no workflow action for non-block_actions payload")
		}
	})

	t.Run("handles URL verification challenge", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		payload := SlackInteractionPayload{
			Type:      "url_verification",
			Challenge: "test-challenge-123",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/callbacks/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		server.HandleSlackCallback(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		if response["challenge"] != "test-challenge-123" {
			t.Errorf("expected challenge 'test-challenge-123', got '%s'", response["challenge"])
		}
	})
}

func TestSignatureValidation(t *testing.T) {
	t.Run("generates consistent Duo signatures", func(t *testing.T) {
		server := NewServer(nil, &Config{Secret: "test-secret"})
		data := []byte(`{"request_id":"req-123"}`)

		sig1 := server.generateDuoSignature(data)
		sig2 := server.generateDuoSignature(data)

		if sig1 != sig2 {
			t.Error("expected signatures to be consistent for same data")
		}
	})

	t.Run("generates different signatures for different data", func(t *testing.T) {
		server := NewServer(nil, &Config{Secret: "test-secret"})
		data1 := []byte(`{"request_id":"req-123"}`)
		data2 := []byte(`{"request_id":"req-456"}`)

		sig1 := server.generateDuoSignature(data1)
		sig2 := server.generateDuoSignature(data2)

		if sig1 == sig2 {
			t.Error("expected different signatures for different data")
		}
	})

	t.Run("validates Duo signature correctly", func(t *testing.T) {
		server := NewServer(nil, &Config{Secret: "test-secret"})
		data := []byte(`{"request_id":"req-123"}`)
		signature := server.generateDuoSignature(data)

		if !server.validateDuoSignature(data, signature) {
			t.Error("expected valid signature to pass validation")
		}
	})

	t.Run("rejects invalid Duo signature", func(t *testing.T) {
		server := NewServer(nil, &Config{Secret: "test-secret"})
		data := []byte(`{"request_id":"req-123"}`)

		if server.validateDuoSignature(data, "invalid-signature") {
			t.Error("expected invalid signature to fail validation")
		}
	})
}

func TestRoutes(t *testing.T) {
	t.Run("SetupRoutes registers all handlers", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)
		mux := http.NewServeMux()
		server.SetupRoutes(mux)

		testCases := []struct {
			path       string
			method     string
			expectCode int
		}{
			{"/callbacks/duo", http.MethodPost, http.StatusBadRequest}, // missing body
			{"/callbacks/slack", http.MethodPost, http.StatusBadRequest},
			{"/callbacks/duo", http.MethodGet, http.StatusMethodNotAllowed},
		}

		for _, tc := range testCases {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.expectCode {
				t.Errorf("%s %s: expected status %d, got %d", tc.method, tc.path, tc.expectCode, rr.Code)
			}
		}
	})
}

func TestResponseCallbackIntegration(t *testing.T) {
	t.Run("notification ResponseCallback is properly structured", func(t *testing.T) {
		callback := notification.ResponseCallback{
			NotificationID: "notif-123",
			RequestID:      "req-123",
			Response:       notification.ResponseApproved,
			RespondedBy:    "user@example.com",
			RespondedAt:    time.Now(),
			Reason:         "Approved via callback",
			Metadata: map[string]string{
				"source": "duo",
			},
		}

		if callback.Response != notification.ResponseApproved {
			t.Errorf("expected response '%s', got '%s'", notification.ResponseApproved, callback.Response)
		}
		if callback.Metadata["source"] != "duo" {
			t.Errorf("expected metadata source 'duo', got '%s'", callback.Metadata["source"])
		}
	})
}

func TestServerClose(t *testing.T) {
	t.Run("Close cleans up resources", func(t *testing.T) {
		engine := &MockWorkflowEngine{}
		server := NewServer(engine, nil)

		err := server.Close()
		if err != nil {
			t.Errorf("unexpected error during close: %v", err)
		}
	})
}
