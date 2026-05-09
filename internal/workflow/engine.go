package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Storage defines the interface for request storage
type Storage interface {
	SaveRequest(req *RequestState) error
	GetRequest(id string) (*RequestState, error)
	UpdateRequest(req *RequestState) error
	ListRequestsByStatus(status string) ([]*RequestState, error)
	DeleteRequest(id string) error
}

// CreateRequestInput contains the input for creating a request
type CreateRequestInput struct {
	ClientID          string
	SecretPath        string
	Reason            string
	RequestedDuration time.Duration
	RequiredApprovals int // Override from policy if > 0
}

// CreateRequestResult contains the result of creating a request
type CreateRequestResult struct {
	RequestID         string
	Status            string
	RequiredApprovals int
	ExpiresAt         time.Time
}

// ApprovalInput contains the input for approving a request
type ApprovalInput struct {
	Identity string
	Reason   string
}

// ApprovalResult contains the result of approving a request
type ApprovalResult struct {
	RequestID  string
	Status     string
	Token      string
	ApprovalCount int
	RequiredApprovals int
}

// DenialInput contains the input for denying a request
type DenialInput struct {
	Identity string
	Reason   string
}

// DenialResult contains the result of denying a request
type DenialResult struct {
	RequestID string
	Status    string
}

// GetStatusResult contains the full status of a request
type GetStatusResult struct {
	RequestID         string
	Status            string
	ClientID          string
	SecretPath        string
	Reason            string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RequiredApprovals int
	ApprovalCount     int
	DenialCount       int
	Approvals         []ApprovalInfo
	Denials           []DenialInfo
	Token             string
	TokenExpiresAt    *time.Time
}

// PolicyEngineInterface defines the interface the workflow engine needs from policy engine
type PolicyEngineInterface interface {
	GetEffectivePolicy(config PolicyConfig, path string) PolicyResult
}

// WorkflowEngine manages the approval workflow lifecycle
type WorkflowEngine struct {
	storage       Storage
	policyEngine  PolicyEngineInterface
	config        PolicyConfig
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(storage Storage, policyEngine PolicyEngineInterface) *WorkflowEngine {
	return &WorkflowEngine{
		storage:      storage,
		policyEngine: policyEngine,
		config: PolicyConfig{
			DefaultRequiredApprovals: 1,
			DefaultMaxDuration:       24 * time.Hour,
		},
	}
}

// NewWorkflowEngineWithConfig creates a workflow engine with custom configuration
func NewWorkflowEngineWithConfig(storage Storage, policyEngine PolicyEngineInterface, config PolicyConfig) *WorkflowEngine {
	return &WorkflowEngine{
		storage:      storage,
		policyEngine: policyEngine,
		config:       config,
	}
}

// CreateRequest creates a new secret access request
func (we *WorkflowEngine) CreateRequest(input CreateRequestInput) (*CreateRequestResult, error) {
	// Get policy for this path
	policy := we.policyEngine.GetEffectivePolicy(we.config, input.SecretPath)

	// Determine required approvals
	requiredApprovals := policy.RequiredApprovals
	if input.RequiredApprovals > 0 {
		requiredApprovals = input.RequiredApprovals
	}

	// Generate request ID
	requestID := generateRequestID()

	// Calculate expiration time
	expiresAt := time.Now().Add(5 * time.Minute) // Default request timeout

	// Create request state
	req := &RequestState{
		RequestID:         requestID,
		Status:            "pending",
		ClientID:          input.ClientID,
		SecretPath:        input.SecretPath,
		Reason:            input.Reason,
		CreatedAt:         time.Now(),
		ExpiresAt:         expiresAt,
		RequiredApprovals: requiredApprovals,
		ApprovalCount:     0,
		DenialCount:       0,
		Approvals:         []ApprovalInfo{},
		Denials:           []DenialInfo{},
	}

	// Save to storage
	if err := we.storage.SaveRequest(req); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return &CreateRequestResult{
		RequestID:         requestID,
		Status:            "pending",
		RequiredApprovals: requiredApprovals,
		ExpiresAt:         expiresAt,
	}, nil
}

// ApproveRequest processes an approval for a request
func (we *WorkflowEngine) ApproveRequest(requestID string, input ApprovalInput) (*ApprovalResult, error) {
	// Get current request state
	req, err := we.storage.GetRequest(requestID)
	if err != nil {
		return nil, fmt.Errorf("request not found: %w", err)
	}

	// Check if request can be approved
	status := &RequestStatus{
		Status:    req.Status,
		ExpiresAt: req.ExpiresAt,
	}

	if !status.CanBeApproved() {
		if status.IsExpired() {
			return nil, fmt.Errorf("request has expired")
		}
		return nil, fmt.Errorf("request cannot be approved in state: %s", req.Status)
	}

	// Check for duplicate approval
	if req.HasApproved(input.Identity) {
		return nil, fmt.Errorf("identity %s has already approved this request", input.Identity)
	}

	// Apply approval
	isComplete := req.ApplyApproval(input.Identity, input.Reason)

	result := &ApprovalResult{
		RequestID:         requestID,
		Status:            req.Status,
		ApprovalCount:     req.ApprovalCount,
		RequiredApprovals: req.RequiredApprovals,
	}

	// If complete, generate token
	if isComplete {
		// In real implementation, this would call the token service
		token := generateToken()
		result.Token = token
		req.Token = token
		now := time.Now()
		req.TokenExpiresAt = &now
	}

	// Update storage
	if err := we.storage.UpdateRequest(req); err != nil {
		return nil, fmt.Errorf("failed to update request: %w", err)
	}

	return result, nil
}

// DenyRequest denies a request
func (we *WorkflowEngine) DenyRequest(requestID string, input DenialInput) (*DenialResult, error) {
	// Get current request state
	req, err := we.storage.GetRequest(requestID)
	if err != nil {
		return nil, fmt.Errorf("request not found: %w", err)
	}

	// Check if request can be denied
	if req.Status != "pending" {
		return nil, fmt.Errorf("request cannot be denied in state: %s", req.Status)
	}

	// Check if request has expired
	status := &RequestStatus{
		Status:    req.Status,
		ExpiresAt: req.ExpiresAt,
	}
	if status.IsExpired() {
		return nil, fmt.Errorf("request has expired")
	}

	// Check for duplicate denial
	if req.HasDenied(input.Identity) {
		return nil, fmt.Errorf("identity %s has already denied this request", input.Identity)
	}

	// Apply denial
	req.ApplyDenial(input.Identity, input.Reason)

	// Update storage
	if err := we.storage.UpdateRequest(req); err != nil {
		return nil, fmt.Errorf("failed to update request: %w", err)
	}

	return &DenialResult{
		RequestID: requestID,
		Status:    "denied",
	}, nil
}

// GetRequestStatus returns the full status of a request
func (we *WorkflowEngine) GetRequestStatus(requestID string) (*GetStatusResult, error) {
	req, err := we.storage.GetRequest(requestID)
	if err != nil {
		return nil, fmt.Errorf("request not found: %w", err)
	}

	return &GetStatusResult{
		RequestID:         req.RequestID,
		Status:            req.Status,
		ClientID:          req.ClientID,
		SecretPath:        req.SecretPath,
		Reason:            req.Reason,
		CreatedAt:         req.CreatedAt,
		ExpiresAt:         req.ExpiresAt,
		RequiredApprovals: req.RequiredApprovals,
		ApprovalCount:     req.ApprovalCount,
		DenialCount:       req.DenialCount,
		Approvals:         req.Approvals,
		Denials:           req.Denials,
		Token:             req.Token,
		TokenExpiresAt:    req.TokenExpiresAt,
	}, nil
}

// ExpireRequest marks a request as expired
func (we *WorkflowEngine) ExpireRequest(requestID string) error {
	req, err := we.storage.GetRequest(requestID)
	if err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	if !req.CanTransitionTo("expired") {
		return fmt.Errorf("request cannot be expired in state: %s", req.Status)
	}

	req.ApplyExpiration()

	if err := we.storage.UpdateRequest(req); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	return nil
}

// ListPendingRequests returns all pending requests
func (we *WorkflowEngine) ListPendingRequests() ([]*RequestState, error) {
	return we.storage.ListRequestsByStatus("pending")
}

// GetApproversForPath returns the approver groups for a given path
func (we *WorkflowEngine) GetApproversForPath(path string) []string {
	policy := we.policyEngine.GetEffectivePolicy(we.config, path)
	return policy.ApproverGroups
}

// UpdateConfig updates the policy configuration
func (we *WorkflowEngine) UpdateConfig(config PolicyConfig) {
	we.config = config
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Generate 8 random bytes for uniqueness
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("req_%s", hex.EncodeToString(b))
}

// generateToken generates a placeholder token
// In real implementation, this would call the token service
func generateToken() string {
	// Generate 16 random bytes for token
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("tok_%s", hex.EncodeToString(b))
}
