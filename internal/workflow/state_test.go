package workflow

import (
	"testing"
	"time"
)

// StateMachine Tests
// These tests verify the request state machine transitions correctly

func TestStateMachine_CanTransition_ValidTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Test: Pending request can be approved
	if !sm.CanTransition("pending", "approved") {
		t.Error("Expected pending -> approved to be valid")
	}

	// Test: Pending request can be denied
	if !sm.CanTransition("pending", "denied") {
		t.Error("Expected pending -> denied to be valid")
	}

	// Test: Pending request can expire
	if !sm.CanTransition("pending", "expired") {
		t.Error("Expected pending -> expired to be valid")
	}
}

func TestStateMachine_CanTransition_InvalidTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Test: Approved request cannot be denied
	if sm.CanTransition("approved", "denied") {
		t.Error("Expected approved -> denied to be invalid")
	}

	// Test: Denied request cannot be approved
	if sm.CanTransition("denied", "approved") {
		t.Error("Expected denied -> approved to be invalid")
	}

	// Test: Expired request cannot be approved
	if sm.CanTransition("expired", "approved") {
		t.Error("Expected expired -> approved to be invalid")
	}

	// Test: Terminal states cannot transition to anything
	if sm.CanTransition("approved", "pending") {
		t.Error("Expected approved -> pending to be invalid")
	}
}

func TestStateMachine_Transition_Success(t *testing.T) {
	sm := NewStateMachine()

	// Test: Successfully transition from pending to approved
	newState, err := sm.Transition("pending", "approved")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if newState != "approved" {
		t.Errorf("Expected state 'approved', got: %s", newState)
	}

	// Test: Successfully transition from pending to denied
	newState, err = sm.Transition("pending", "denied")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if newState != "denied" {
		t.Errorf("Expected state 'denied', got: %s", newState)
	}
}

func TestStateMachine_Transition_Failure(t *testing.T) {
	sm := NewStateMachine()

	// Test: Cannot transition from approved to denied
	_, err := sm.Transition("approved", "denied")
	if err == nil {
		t.Error("Expected error for invalid transition approved -> denied")
	}

	// Test: Cannot transition from denied to approved
	_, err = sm.Transition("denied", "approved")
	if err == nil {
		t.Error("Expected error for invalid transition denied -> approved")
	}
}

func TestStateMachine_IsTerminal(t *testing.T) {
	sm := NewStateMachine()

	// Test: Terminal states
	if !sm.IsTerminal("approved") {
		t.Error("Expected 'approved' to be terminal")
	}
	if !sm.IsTerminal("denied") {
		t.Error("Expected 'denied' to be terminal")
	}
	if !sm.IsTerminal("expired") {
		t.Error("Expected 'expired' to be terminal")
	}

	// Test: Non-terminal state
	if sm.IsTerminal("pending") {
		t.Error("Expected 'pending' to NOT be terminal")
	}
}

func TestStateMachine_GetValidTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Test: Valid transitions from pending
	transitions := sm.GetValidTransitions("pending")
	if len(transitions) != 3 {
		t.Errorf("Expected 3 transitions from pending, got: %d", len(transitions))
	}

	expected := map[string]bool{"approved": true, "denied": true, "expired": true}
	for _, transition := range transitions {
		if !expected[transition] {
			t.Errorf("Unexpected transition: %s", transition)
		}
	}

	// Test: Valid transitions from approved (should be empty)
	transitions = sm.GetValidTransitions("approved")
	if len(transitions) != 0 {
		t.Errorf("Expected 0 transitions from approved, got: %d", len(transitions))
	}
}

// RequestStatus Tests
// These tests verify request status checking with expiration

func TestRequestStatus_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "Request already expired",
			expiresAt: now.Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "Request expiring now",
			expiresAt: now,
			want:      true,
		},
		{
			name:      "Request not yet expired",
			expiresAt: now.Add(1 * time.Hour),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestStatus{
				Status:    "pending",
				ExpiresAt: tt.expiresAt,
			}
			if got := req.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestStatus_CanBeApproved(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		status string
		expiry time.Time
		want   bool
	}{
		{
			name:   "Pending request not expired",
			status: "pending",
			expiry: now.Add(1 * time.Hour),
			want:   true,
		},
		{
			name:   "Pending request expired",
			status: "pending",
			expiry: now.Add(-1 * time.Hour),
			want:   false,
		},
		{
			name:   "Already approved",
			status: "approved",
			expiry: now.Add(1 * time.Hour),
			want:   false,
		},
		{
			name:   "Already denied",
			status: "denied",
			expiry: now.Add(1 * time.Hour),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestStatus{
				Status:    tt.status,
				ExpiresAt: tt.expiry,
			}
			if got := req.CanBeApproved(); got != tt.want {
				t.Errorf("CanBeApproved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestStatus_ShouldAutoExpire(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		status string
		expiry time.Time
		want   bool
	}{
		{
			name:   "Pending and expired should auto-expire",
			status: "pending",
			expiry: now.Add(-1 * time.Minute),
			want:   true,
		},
		{
			name:   "Pending but not expired",
			status: "pending",
			expiry: now.Add(1 * time.Hour),
			want:   false,
		},
		{
			name:   "Already approved should not auto-expire",
			status: "approved",
			expiry: now.Add(-1 * time.Hour),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestStatus{
				Status:    tt.status,
				ExpiresAt: tt.expiry,
			}
			if got := req.ShouldAutoExpire(); got != tt.want {
				t.Errorf("ShouldAutoExpire() = %v, want %v", got, tt.want)
			}
		})
	}
}

// RequestState Tests
// These tests verify the full request state management

func TestRequestState_ApplyApproval(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name             string
		initialApprovals int
		requiredApprovals int
		wantApproved     bool
	}{
		{
			name:             "Single approval meets requirement",
			initialApprovals: 0,
			requiredApprovals: 1,
			wantApproved:     true,
		},
		{
			name:             "Multiple approvals needed",
			initialApprovals: 0,
			requiredApprovals: 3,
			wantApproved:     false,
		},
		{
			name:             "Quorum reached with multiple approvers",
			initialApprovals: 2,
			requiredApprovals: 3,
			wantApproved:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &RequestState{
				Status:            "pending",
				ApprovalCount:     tt.initialApprovals,
				RequiredApprovals: tt.requiredApprovals,
				ExpiresAt:         now.Add(1 * time.Hour),
			}

			isComplete := rs.ApplyApproval("approver-1", "LGTM")

			if isComplete != tt.wantApproved {
				t.Errorf("ApplyApproval() complete = %v, want %v", isComplete, tt.wantApproved)
			}

			if rs.ApprovalCount != tt.initialApprovals+1 {
				t.Errorf("Expected approval count %d, got %d", tt.initialApprovals+1, rs.ApprovalCount)
			}
		})
	}
}

func TestRequestState_ApplyDenial(t *testing.T) {
	now := time.Now()
	rs := &RequestState{
		Status:    "pending",
		ExpiresAt: now.Add(1 * time.Hour),
	}

	isComplete := rs.ApplyDenial("denier-1", "Not authorized")

	if !isComplete {
		t.Error("Expected denial to immediately complete request")
	}

	if rs.Status != "denied" {
		t.Errorf("Expected status 'denied', got: %s", rs.Status)
	}
}

func TestRequestState_ApplyExpiration(t *testing.T) {
	now := time.Now()
	rs := &RequestState{
		Status:    "pending",
		ExpiresAt: now.Add(-1 * time.Hour),
	}

	rs.ApplyExpiration()

	if rs.Status != "expired" {
		t.Errorf("Expected status 'expired', got: %s", rs.Status)
	}
}

func TestRequestState_GetApproverIdentities(t *testing.T) {
	rs := &RequestState{
		Approvals: []ApprovalInfo{
			{Identity: "alice", Reason: "Approved"},
			{Identity: "bob", Reason: "LGTM"},
		},
	}

	identities := rs.GetApproverIdentities()

	if len(identities) != 2 {
		t.Errorf("Expected 2 approvers, got: %d", len(identities))
	}

	if identities[0] != "alice" || identities[1] != "bob" {
		t.Errorf("Unexpected approvers: %v", identities)
	}
}

func TestRequestState_HasApproved(t *testing.T) {
	rs := &RequestState{
		Approvals: []ApprovalInfo{
			{Identity: "alice", Reason: "Approved"},
		},
	}

	if !rs.HasApproved("alice") {
		t.Error("Expected HasApproved('alice') to return true")
	}

	if rs.HasApproved("bob") {
		t.Error("Expected HasApproved('bob') to return false")
	}
}
