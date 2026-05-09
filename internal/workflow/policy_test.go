package workflow

import (
	"testing"
	"time"
)

// Policy Engine Tests
// These tests verify path-based policy rules are evaluated correctly

func TestPolicyEngine_Evaluate_MatchesPath(t *testing.T) {
	engine := NewPolicyEngine()

	// Test: Exact path match
	policy := PolicyRule{
		PathPattern: "prod/database/credentials",
		RequiredApprovals: 2,
	}
	if !engine.MatchesPath(policy, "prod/database/credentials") {
		t.Error("Expected exact path to match")
	}

	// Test: Wildcard path match
	policy = PolicyRule{
		PathPattern: "prod/*",
		RequiredApprovals: 1,
	}
	if !engine.MatchesPath(policy, "prod/database/credentials") {
		t.Error("Expected wildcard pattern 'prod/*' to match 'prod/database/credentials'")
	}

	// Test: No match
	if engine.MatchesPath(policy, "staging/database/credentials") {
		t.Error("Expected 'prod/*' to NOT match 'staging/database/credentials'")
	}
}

func TestPolicyEngine_Evaluate_Specificity(t *testing.T) {
	engine := NewPolicyEngine()

	policies := []PolicyRule{
		{
			PathPattern:       "*",
			RequiredApprovals: 1,
		},
		{
			PathPattern:       "prod/*",
			RequiredApprovals: 2,
		},
		{
			PathPattern:       "prod/database/*",
			RequiredApprovals: 3,
		},
	}

	// Test: Most specific policy wins
	result := engine.Evaluate(policies, "prod/database/credentials")
	if result.RequiredApprovals != 3 {
		t.Errorf("Expected 3 approvals for prod/database/*, got: %d", result.RequiredApprovals)
	}

	// Test: Fallback to less specific
	result = engine.Evaluate(policies, "prod/api-keys")
	if result.RequiredApprovals != 2 {
		t.Errorf("Expected 2 approvals for prod/*, got: %d", result.RequiredApprovals)
	}

	// Test: Default policy
	result = engine.Evaluate(policies, "staging/secrets")
	if result.RequiredApprovals != 1 {
		t.Errorf("Expected 1 approval for default policy, got: %d", result.RequiredApprovals)
	}
}

func TestPolicyEngine_Evaluate_MaxDuration(t *testing.T) {
	engine := NewPolicyEngine()

	policies := []PolicyRule{
		{
			PathPattern:  "prod/*",
			MaxDuration:  1 * time.Hour,
		},
	}

	// Test: Duration within limit
	result := engine.Evaluate(policies, "prod/secrets")
	if result.MaxDuration != 1*time.Hour {
		t.Errorf("Expected max duration 1h, got: %v", result.MaxDuration)
	}
}

func TestPolicyEngine_Evaluate_ApproverGroups(t *testing.T) {
	engine := NewPolicyEngine()

	policies := []PolicyRule{
		{
			PathPattern:    "prod/database/*",
			ApproverGroups: []string{"dba-team", "sre-team"},
		},
	}

	result := engine.Evaluate(policies, "prod/database/credentials")

	if len(result.ApproverGroups) != 2 {
		t.Errorf("Expected 2 approver groups, got: %d", len(result.ApproverGroups))
	}

	expectedGroups := map[string]bool{"dba-team": true, "sre-team": true}
	for _, group := range result.ApproverGroups {
		if !expectedGroups[group] {
			t.Errorf("Unexpected approver group: %s", group)
		}
	}
}

func TestPolicyEngine_IsAllowedApprover(t *testing.T) {
	engine := NewPolicyEngine()

	policy := PolicyResult{
		ApproverGroups: []string{"admin-team", "dev-team"},
	}

	memberProvider := &MockGroupMembershipProvider{
		memberships: map[string][]string{
			"alice": {"admin-team"},
			"bob":   {"dev-team"},
			"charlie": {"other-team"},
		},
	}

	// Test: Member of allowed group
	if !engine.IsAllowedApprover(policy, "alice", memberProvider) {
		t.Error("Expected alice to be allowed (member of admin-team)")
	}

	// Test: Member of another allowed group
	if !engine.IsAllowedApprover(policy, "bob", memberProvider) {
		t.Error("Expected bob to be allowed (member of dev-team)")
	}

	// Test: Not a member of any allowed group
	if engine.IsAllowedApprover(policy, "charlie", memberProvider) {
		t.Error("Expected charlie to NOT be allowed (not in approver groups)")
	}
}

func TestPolicyEngine_ValidateDuration(t *testing.T) {
	engine := NewPolicyEngine()

	policy := PolicyResult{
		MaxDuration: 1 * time.Hour,
	}

	// Test: Duration within limit
	if err := engine.ValidateDuration(policy, 30*time.Minute); err != nil {
		t.Errorf("Expected 30m to be valid, got error: %v", err)
	}

	// Test: Duration at limit
	if err := engine.ValidateDuration(policy, 1*time.Hour); err != nil {
		t.Errorf("Expected 1h to be valid, got error: %v", err)
	}

	// Test: Duration exceeds limit
	if err := engine.ValidateDuration(policy, 2*time.Hour); err == nil {
		t.Error("Expected 2h to exceed limit and return error")
	}
}

func TestPolicyEngine_GetEffectivePolicy(t *testing.T) {
	engine := NewPolicyEngine()

	config := PolicyConfig{
		Rules: []PolicyRule{
			{
				PathPattern:       "*",
				RequiredApprovals: 1,
				MaxDuration:       24 * time.Hour,
			},
			{
				PathPattern:       "prod/*",
				RequiredApprovals: 2,
				MaxDuration:       1 * time.Hour,
				ApproverGroups:    []string{"sre-team"},
			},
		},
		DefaultRequiredApprovals: 1,
	}

	// Test: Production policy applied
	policy := engine.GetEffectivePolicy(config, "prod/database/credentials")
	if policy.RequiredApprovals != 2 {
		t.Errorf("Expected 2 approvals for prod, got: %d", policy.RequiredApprovals)
	}
	if policy.MaxDuration != 1*time.Hour {
		t.Errorf("Expected 1h max duration for prod, got: %v", policy.MaxDuration)
	}

	// Test: Default policy applied
	policy = engine.GetEffectivePolicy(config, "staging/secrets")
	if policy.RequiredApprovals != 1 {
		t.Errorf("Expected 1 approval for default, got: %d", policy.RequiredApprovals)
	}
}

func TestPolicyEngine_MultipleMatchingPolicies(t *testing.T) {
	engine := NewPolicyEngine()

	policies := []PolicyRule{
		{
			PathPattern:       "prod/*",
			RequiredApprovals: 2,
			MaxDuration:       4 * time.Hour,
		},
		{
			PathPattern:       "prod/database/*",
			RequiredApprovals: 3,
			MaxDuration:       2 * time.Hour,
			ApproverGroups:    []string{"dba-team"},
		},
		{
			PathPattern:       "prod/database/credentials",
			RequiredApprovals: 4,
			MaxDuration:       1 * time.Hour,
			ApproverGroups:    []string{"dba-team", "security-team"},
		},
	}

	// Test: Most specific match (exact path)
	result := engine.Evaluate(policies, "prod/database/credentials")
	if result.RequiredApprovals != 4 {
		t.Errorf("Expected 4 approvals for exact match, got: %d", result.RequiredApprovals)
	}
	if result.MaxDuration != 1*time.Hour {
		t.Errorf("Expected 1h duration for exact match, got: %v", result.MaxDuration)
	}
	if len(result.ApproverGroups) != 2 {
		t.Errorf("Expected 2 approver groups for exact match, got: %d", len(result.ApproverGroups))
	}

	// Test: Next most specific (wildcard match)
	result = engine.Evaluate(policies, "prod/database/config")
	if result.RequiredApprovals != 3 {
		t.Errorf("Expected 3 approvals for prod/database/*, got: %d", result.RequiredApprovals)
	}
}

func TestPolicyEngine_EmptyPolicies(t *testing.T) {
	engine := NewPolicyEngine()

	// Test: Empty policies should return default result
	result := engine.Evaluate([]PolicyRule{}, "any/path")

	if result.RequiredApprovals != 1 {
		t.Errorf("Expected default 1 approval, got: %d", result.RequiredApprovals)
	}
	if result.MaxDuration != 0 {
		t.Errorf("Expected no max duration limit, got: %v", result.MaxDuration)
	}
	if len(result.ApproverGroups) != 0 {
		t.Errorf("Expected no approver groups, got: %d", len(result.ApproverGroups))
	}
}

// MockGroupMembershipProvider for testing
type MockGroupMembershipProvider struct {
	memberships map[string][]string
}

func (m *MockGroupMembershipProvider) GetUserGroups(identity string) []string {
	return m.memberships[identity]
}
