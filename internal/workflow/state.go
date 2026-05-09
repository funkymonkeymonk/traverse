package workflow

import (
	"fmt"
	"time"
)

// StateMachine manages request state transitions
type StateMachine struct {
	transitions map[string]map[string]bool
	terminalStates map[string]bool
}

// NewStateMachine creates a new state machine with valid transitions
func NewStateMachine() *StateMachine {
	return &StateMachine{
		transitions: map[string]map[string]bool{
			"pending": {
				"approved": true,
				"denied":   true,
				"expired":  true,
			},
			"approved": {},
			"denied":   {},
			"expired":  {},
		},
		terminalStates: map[string]bool{
			"approved": true,
			"denied":   true,
			"expired":  true,
		},
	}
}

// CanTransition checks if a transition from one state to another is valid
func (sm *StateMachine) CanTransition(from, to string) bool {
	if validTransitions, ok := sm.transitions[from]; ok {
		return validTransitions[to]
	}
	return false
}

// Transition attempts to transition from one state to another
// Returns the new state and an error if the transition is invalid
func (sm *StateMachine) Transition(from, to string) (string, error) {
	if !sm.CanTransition(from, to) {
		return from, fmt.Errorf("invalid state transition from %s to %s", from, to)
	}
	return to, nil
}

// IsTerminal returns true if the state is terminal (no further transitions allowed)
func (sm *StateMachine) IsTerminal(state string) bool {
	return sm.terminalStates[state]
}

// GetValidTransitions returns all valid target states from the given state
func (sm *StateMachine) GetValidTransitions(from string) []string {
	var result []string
	if validTransitions, ok := sm.transitions[from]; ok {
		for to := range validTransitions {
			result = append(result, to)
		}
	}
	return result
}

// RequestStatus provides methods to check request status with expiration
// This is a lightweight view for status checks
type RequestStatus struct {
	Status    string
	ExpiresAt time.Time
}

// IsExpired returns true if the request has expired
func (rs *RequestStatus) IsExpired() bool {
	return time.Now().After(rs.ExpiresAt) || time.Now().Equal(rs.ExpiresAt)
}

// CanBeApproved returns true if the request can be approved
// (must be pending and not expired)
func (rs *RequestStatus) CanBeApproved() bool {
	return rs.Status == "pending" && !rs.IsExpired()
}

// ShouldAutoExpire returns true if the request is pending and has expired
func (rs *RequestStatus) ShouldAutoExpire() bool {
	return rs.Status == "pending" && rs.IsExpired()
}

// ApprovalInfo contains information about an approval
type ApprovalInfo struct {
	Identity   string
	ApprovedAt time.Time
	Reason     string
}

// DenialInfo contains information about a denial
type DenialInfo struct {
	Identity string
	DeniedAt time.Time
	Reason   string
}

// RequestState manages the full state of a request including approvals and denials
type RequestState struct {
	RequestID         string
	Status            string
	ClientID          string
	SecretPath        string
	Reason            string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	ApprovedAt        *time.Time
	DeniedAt          *time.Time
	RequiredApprovals int
	ApprovalCount     int
	DenialCount       int
	Approvals         []ApprovalInfo
	Denials           []DenialInfo
	Token             string
	TokenExpiresAt    *time.Time
}

// ApplyApproval adds an approval and checks if the request is complete
// Returns true if the quorum has been reached
func (rs *RequestState) ApplyApproval(identity, reason string) bool {
	rs.ApprovalCount++
	rs.Approvals = append(rs.Approvals, ApprovalInfo{
		Identity:   identity,
		ApprovedAt: time.Now(),
		Reason:     reason,
	})

	if rs.ApprovalCount >= rs.RequiredApprovals {
		rs.Status = "approved"
		now := time.Now()
		rs.ApprovedAt = &now
		return true
	}

	return false
}

// ApplyDenial adds a denial and marks the request as denied
// Returns true as denial immediately completes the request
func (rs *RequestState) ApplyDenial(identity, reason string) bool {
	rs.DenialCount++
	rs.Denials = append(rs.Denials, DenialInfo{
		Identity: identity,
		DeniedAt: time.Now(),
		Reason:   reason,
	})

	rs.Status = "denied"
	now := time.Now()
	rs.DeniedAt = &now
	return true
}

// ApplyExpiration marks the request as expired
func (rs *RequestState) ApplyExpiration() {
	rs.Status = "expired"
}

// GetApproverIdentities returns a list of all approver identities
func (rs *RequestState) GetApproverIdentities() []string {
	var identities []string
	for _, approval := range rs.Approvals {
		identities = append(identities, approval.Identity)
	}
	return identities
}

// HasApproved returns true if the given identity has already approved
func (rs *RequestState) HasApproved(identity string) bool {
	for _, approval := range rs.Approvals {
		if approval.Identity == identity {
			return true
		}
	}
	return false
}

// HasDenied returns true if the given identity has already denied
func (rs *RequestState) HasDenied(identity string) bool {
	for _, denial := range rs.Denials {
		if denial.Identity == identity {
			return true
		}
	}
	return false
}

// IsResolved returns true if the request is in a terminal state
func (rs *RequestState) IsResolved() bool {
	return rs.Status == "approved" || rs.Status == "denied" || rs.Status == "expired"
}

// CanTransitionTo returns true if the request can transition to the given state
func (rs *RequestState) CanTransitionTo(newStatus string) bool {
	sm := NewStateMachine()
	return sm.CanTransition(rs.Status, newStatus)
}
