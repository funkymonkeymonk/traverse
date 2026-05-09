package workflow

import (
	"testing"
	"time"
)

// Workflow Engine Tests
// These tests verify the complete approval workflow lifecycle

func TestWorkflowEngine_CreateRequest_Success(t *testing.T) {
	storage := NewMockStorage()
	policyEngine := &MockPolicyEngine{
		policy: PolicyResult{
			RequiredApprovals: 1,
		},
	}
	engine := NewWorkflowEngine(storage, policyEngine)

	req := CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/database/credentials",
		Reason:        "Need access for database migration",
		RequestedDuration: 30 * time.Minute,
	}

	result, err := engine.CreateRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.RequestID == "" {
		t.Error("Expected request ID to be generated")
	}

	if result.Status != "pending" {
		t.Errorf("Expected status 'pending', got: %s", result.Status)
	}

	if result.RequiredApprovals != 1 {
		t.Errorf("Expected 1 required approval, got: %d", result.RequiredApprovals)
	}

	// Verify request was stored
	stored, err := storage.GetRequest(result.RequestID)
	if err != nil {
		t.Fatalf("Expected request to be stored, got error: %v", err)
	}

	if stored.Status != "pending" {
		t.Errorf("Expected stored status 'pending', got: %s", stored.Status)
	}
}

func TestWorkflowEngine_CreateRequest_WithPolicy(t *testing.T) {
	storage := NewMockStorage()
	policyEngine := &MockPolicyEngine{
		policy: PolicyResult{
			RequiredApprovals: 3,
			MaxDuration:       1 * time.Hour,
			ApproverGroups:    []string{"sre-team"},
		},
	}
	engine := NewWorkflowEngine(storage, policyEngine)

	req := CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/database/credentials",
		Reason:        "Need access for emergency fix",
		RequestedDuration: 30 * time.Minute,
	}

	result, err := engine.CreateRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.RequiredApprovals != 3 {
		t.Errorf("Expected 3 required approvals from policy, got: %d", result.RequiredApprovals)
	}
}

func TestWorkflowEngine_ApproveRequest_SingleApprover(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	// Create request
	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test approval",
		RequestedDuration: 1 * time.Hour,
	})

	// Approve request
	result, err := engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "LGTM",
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != "approved" {
		t.Errorf("Expected status 'approved', got: %s", result.Status)
	}

	if result.Token == "" {
		t.Error("Expected token to be issued")
	}

	if result.ApprovalCount != 1 {
		t.Errorf("Expected 1 approval, got: %d", result.ApprovalCount)
	}
}

func TestWorkflowEngine_ApproveRequest_MultiApprover(t *testing.T) {
	storage := NewMockStorage()
	policyEngine := &MockPolicyEngine{
		policy: PolicyResult{
			RequiredApprovals: 3,
		},
	}
	engine := NewWorkflowEngine(storage, policyEngine)

	// Create request requiring 3 approvals
	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/database/credentials",
		Reason:        "Critical production access",
		RequestedDuration: 1 * time.Hour,
	})

	// First approval - should not complete
	result, _ := engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "First approval",
	})

	if result.Status != "pending" {
		t.Errorf("Expected status 'pending' after 1 approval, got: %s", result.Status)
	}

	if result.Token != "" {
		t.Error("Expected no token yet")
	}

	// Second approval - should not complete
	result, _ = engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-2",
		Reason:   "Second approval",
	})

	if result.Status != "pending" {
		t.Errorf("Expected status 'pending' after 2 approvals, got: %s", result.Status)
	}

	// Third approval - should complete
	result, _ = engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-3",
		Reason:   "Third approval - quorum reached",
	})

	if result.Status != "approved" {
		t.Errorf("Expected status 'approved' after 3 approvals, got: %s", result.Status)
	}

	if result.Token == "" {
		t.Error("Expected token to be issued after quorum reached")
	}

	if result.ApprovalCount != 3 {
		t.Errorf("Expected 3 approvals, got: %d", result.ApprovalCount)
	}
}

func TestWorkflowEngine_ApproveRequest_DuplicateApprover(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test duplicate",
		RequestedDuration: 1 * time.Hour,
	})

	// First approval
	engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "First approval",
	})

	// Duplicate approval - should fail
	_, err := engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Duplicate approval",
	})

	if err == nil {
		t.Error("Expected error for duplicate approval")
	}
}

func TestWorkflowEngine_ApproveRequest_AlreadyDenied(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test denied state",
		RequestedDuration: 1 * time.Hour,
	})

	// Deny the request first
	engine.DenyRequest(req.RequestID, DenialInput{
		Identity: "denier-1",
		Reason:   "Not authorized",
	})

	// Try to approve - should fail
	_, err := engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Trying to approve denied request",
	})

	if err == nil {
		t.Error("Expected error when approving already-denied request")
	}
}

func TestWorkflowEngine_ApproveRequest_Expired(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	// Create an already-expired request
	req := &RequestState{
		RequestID:         "expired-req",
		Status:            "pending",
		ClientID:          "client-1",
		SecretPath:        "prod/secrets",
		Reason:            "Test expired",
		ExpiresAt:         time.Now().Add(-1 * time.Hour),
		RequiredApprovals: 1,
	}
	storage.SaveRequest(req)

	// Try to approve expired request - should fail
	_, err := engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Trying to approve expired request",
	})

	if err == nil {
		t.Error("Expected error when approving expired request")
	}
}

func TestWorkflowEngine_DenyRequest_Success(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test denial",
		RequestedDuration: 1 * time.Hour,
	})

	result, err := engine.DenyRequest(req.RequestID, DenialInput{
		Identity: "denier-1",
		Reason:   "Not authorized for this path",
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != "denied" {
		t.Errorf("Expected status 'denied', got: %s", result.Status)
	}

	// Verify the request was actually denied by checking storage
	storedReq, _ := storage.GetRequest(req.RequestID)
	if storedReq == nil || storedReq.DenialCount != 1 {
		t.Errorf("Expected 1 denial record in storage")
	}
}

func TestWorkflowEngine_DenyRequest_AlreadyApproved(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test already approved",
		RequestedDuration: 1 * time.Hour,
	})

	// Approve first
	engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Approved",
	})

	// Try to deny - should fail
	_, err := engine.DenyRequest(req.RequestID, DenialInput{
		Identity: "denier-1",
		Reason:   "Trying to deny approved request",
	})

	if err == nil {
		t.Error("Expected error when denying already-approved request")
	}
}

func TestWorkflowEngine_GetRequestStatus(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	// Create and approve request
	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test get status",
		RequestedDuration: 1 * time.Hour,
	})

	engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Approved",
	})

	// Get status
	status, err := engine.GetRequestStatus(req.RequestID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if status.Status != "approved" {
		t.Errorf("Expected status 'approved', got: %s", status.Status)
	}

	if status.Token == "" {
		t.Error("Expected token in status")
	}

	if len(status.Approvals) != 1 {
		t.Errorf("Expected 1 approval in status, got: %d", len(status.Approvals))
	}
}

func TestWorkflowEngine_GetRequestStatus_NotFound(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	_, err := engine.GetRequestStatus("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent request")
	}
}

func TestWorkflowEngine_ExpireRequest(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	// Create a request that will be expired
	req, _ := engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Test expiration",
		RequestedDuration: 1 * time.Hour,
	})

	// Manually expire it
	err := engine.ExpireRequest(req.RequestID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check status
	status, _ := engine.GetRequestStatus(req.RequestID)
	if status.Status != "expired" {
		t.Errorf("Expected status 'expired', got: %s", status.Status)
	}

	// Try to approve expired request
	_, err = engine.ApproveRequest(req.RequestID, ApprovalInput{
		Identity: "approver-1",
		Reason:   "Too late",
	})
	if err == nil {
		t.Error("Expected error when trying to approve expired request")
	}
}

func TestWorkflowEngine_ListPendingRequests(t *testing.T) {
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, &MockPolicyEngine{})

	// Create multiple requests
	engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-1",
		SecretPath:    "prod/secrets",
		Reason:        "Request 1",
		RequestedDuration: 1 * time.Hour,
	})

	engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-2",
		SecretPath:    "staging/secrets",
		Reason:        "Request 2",
		RequestedDuration: 1 * time.Hour,
	})

	engine.CreateRequest(CreateRequestInput{
		ClientID:      "client-3",
		SecretPath:    "dev/secrets",
		Reason:        "Request 3",
		RequestedDuration: 1 * time.Hour,
	})

	// List pending requests
	requests, err := engine.ListPendingRequests()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(requests) != 3 {
		t.Errorf("Expected 3 pending requests, got: %d", len(requests))
	}
}

func TestWorkflowEngine_GetApproversForPath(t *testing.T) {
	policyEngine := &MockPolicyEngine{
		policy: PolicyResult{
			ApproverGroups: []string{"sre-team", "security-team"},
		},
	}
	storage := NewMockStorage()
	engine := NewWorkflowEngine(storage, policyEngine)

	groups := engine.GetApproversForPath("prod/secrets")

	if len(groups) != 2 {
		t.Errorf("Expected 2 approver groups, got: %d", len(groups))
	}

	expected := map[string]bool{"sre-team": true, "security-team": true}
	for _, group := range groups {
		if !expected[group] {
			t.Errorf("Unexpected group: %s", group)
		}
	}
}

// Mock implementations for testing

type MockStorage struct {
	requests map[string]*RequestState
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		requests: make(map[string]*RequestState),
	}
}

func (m *MockStorage) SaveRequest(req *RequestState) error {
	m.requests[req.RequestID] = req
	return nil
}

func (m *MockStorage) GetRequest(id string) (*RequestState, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, ErrRequestNotFound
	}
	return req, nil
}

func (m *MockStorage) UpdateRequest(req *RequestState) error {
	m.requests[req.RequestID] = req
	return nil
}

func (m *MockStorage) ListRequestsByStatus(status string) ([]*RequestState, error) {
	var result []*RequestState
	for _, req := range m.requests {
		if req.Status == status {
			result = append(result, req)
		}
	}
	return result, nil
}

func (m *MockStorage) DeleteRequest(id string) error {
	delete(m.requests, id)
	return nil
}

type MockPolicyEngine struct {
	policy PolicyResult
}

func (m *MockPolicyEngine) Evaluate(rules []PolicyRule, path string) PolicyResult {
	return m.policy
}

func (m *MockPolicyEngine) GetEffectivePolicy(config PolicyConfig, path string) PolicyResult {
	return m.policy
}

var ErrRequestNotFound = ErrNotFound{Message: "request not found"}

type ErrNotFound struct {
	Message string
}

func (e ErrNotFound) Error() string {
	return e.Message
}
