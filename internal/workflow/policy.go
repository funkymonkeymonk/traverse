package workflow

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// PolicyRule defines a single policy rule based on path patterns
type PolicyRule struct {
	PathPattern       string
	RequiredApprovals int
	MaxDuration       time.Duration
	ApproverGroups    []string
}

// PolicyResult is the effective policy after evaluation
type PolicyResult struct {
	RequiredApprovals int
	MaxDuration       time.Duration
	ApproverGroups    []string
	MatchedRule       string
}

// PolicyConfig holds all policy rules and defaults
type PolicyConfig struct {
	Rules                    []PolicyRule
	DefaultRequiredApprovals int
	DefaultMaxDuration       time.Duration
}

// GroupMembershipProvider defines the interface for checking group membership
type GroupMembershipProvider interface {
	GetUserGroups(identity string) []string
}

// PolicyEngine evaluates policies against secret paths
type PolicyEngine struct {
	config PolicyConfig
}

// NewPolicyEngine creates a new policy engine with the given configuration
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		config: PolicyConfig{
			DefaultRequiredApprovals: 1,
			DefaultMaxDuration:       0, // No limit by default
		},
	}
}

// NewPolicyEngineWithConfig creates a policy engine with custom config
func NewPolicyEngineWithConfig(config PolicyConfig) *PolicyEngine {
	if config.DefaultRequiredApprovals == 0 {
		config.DefaultRequiredApprovals = 1
	}
	// Note: DefaultMaxDuration of 0 means no limit, which is valid
	return &PolicyEngine{config: config}
}

// MatchesPath checks if a policy rule matches the given path
// Supports glob-style wildcards: * matches any single path segment
func (pe *PolicyEngine) MatchesPath(rule PolicyRule, secretPath string) bool {
	pattern := rule.PathPattern

	// Handle exact match
	if pattern == secretPath {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		return matchWildcard(pattern, secretPath)
	}

	return false
}

// matchWildcard performs glob-style matching
func matchWildcard(pattern, path string) bool {
	// Split both pattern and path into components
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	// Handle trailing wildcards like "prod/*"
	// This should match any path starting with "prod/"
	if len(patternParts) > 0 && patternParts[len(patternParts)-1] == "*" {
		// Pattern like "prod/*" should match "prod/anything" including "prod/anything/more"
		if len(pathParts) < len(patternParts)-1 {
			return false
		}
		// Check all non-wildcard parts match
		for i := 0; i < len(patternParts)-1; i++ {
			if patternParts[i] != pathParts[i] {
				return false
			}
		}
		return true
	}

	// For non-trailing wildcards, require exact component count
	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if part == "*" {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}

	return true
}

// Evaluate finds the most specific matching policy for a given path
// More specific patterns take precedence over less specific ones
func (pe *PolicyEngine) Evaluate(rules []PolicyRule, secretPath string) PolicyResult {
	var bestMatch *PolicyRule
	bestSpecificity := -1

	for i := range rules {
		rule := &rules[i]
		if pe.MatchesPath(*rule, secretPath) {
			specificity := calculateSpecificity(rule.PathPattern)
			if specificity > bestSpecificity {
				bestSpecificity = specificity
				bestMatch = rule
			}
		}
	}

	if bestMatch != nil {
		return PolicyResult{
			RequiredApprovals: bestMatch.RequiredApprovals,
			MaxDuration:       bestMatch.MaxDuration,
			ApproverGroups:    bestMatch.ApproverGroups,
			MatchedRule:       bestMatch.PathPattern,
		}
	}

	// Return default policy
	return PolicyResult{
		RequiredApprovals: pe.config.DefaultRequiredApprovals,
		MaxDuration:       pe.config.DefaultMaxDuration,
		ApproverGroups:    nil,
		MatchedRule:       "default",
	}
}

// calculateSpecificity determines how specific a path pattern is
// Higher values mean more specific (exact matches > wildcards)
func calculateSpecificity(pattern string) int {
	parts := strings.Split(pattern, "/")
	specificity := 0

	for _, part := range parts {
		if part == "*" {
			specificity += 1
		} else {
			specificity += 10
		}
	}

	return specificity
}

// GetEffectivePolicy returns the effective policy for a given path
func (pe *PolicyEngine) GetEffectivePolicy(config PolicyConfig, secretPath string) PolicyResult {
	return pe.Evaluate(config.Rules, secretPath)
}

// IsAllowedApprover checks if an identity is allowed to approve based on policy
func (pe *PolicyEngine) IsAllowedApprover(policy PolicyResult, identity string, groupProvider GroupMembershipProvider) bool {
	// If no approver groups are specified, anyone can approve
	if len(policy.ApproverGroups) == 0 {
		return true
	}

	// Check if user is in any of the allowed groups
	userGroups := groupProvider.GetUserGroups(identity)
	for _, userGroup := range userGroups {
		for _, allowedGroup := range policy.ApproverGroups {
			if userGroup == allowedGroup {
				return true
			}
		}
	}

	return false
}

// ValidateDuration checks if a requested duration is within policy limits
func (pe *PolicyEngine) ValidateDuration(policy PolicyResult, requestedDuration time.Duration) error {
	if policy.MaxDuration == 0 {
		return nil // No limit
	}

	if requestedDuration > policy.MaxDuration {
		return fmt.Errorf("requested duration %v exceeds maximum allowed %v", requestedDuration, policy.MaxDuration)
	}

	return nil
}

// GetMaxDuration returns the maximum allowed duration for a path
func (pe *PolicyEngine) GetMaxDuration(config PolicyConfig, secretPath string) time.Duration {
	policy := pe.GetEffectivePolicy(config, secretPath)
	return policy.MaxDuration
}

// GetRequiredApprovals returns the number of approvals required for a path
func (pe *PolicyEngine) GetRequiredApprovals(config PolicyConfig, secretPath string) int {
	policy := pe.GetEffectivePolicy(config, secretPath)
	return policy.RequiredApprovals
}

// GetApproverGroups returns the list of approver groups for a path
func (pe *PolicyEngine) GetApproverGroups(config PolicyConfig, secretPath string) []string {
	policy := pe.GetEffectivePolicy(config, secretPath)
	return policy.ApproverGroups
}

// ValidatePath validates that a path matches the policy format
func (pe *PolicyEngine) ValidatePath(secretPath string) error {
	if secretPath == "" {
		return fmt.Errorf("secret path cannot be empty")
	}

	// Clean the path to normalize it
	cleaned := path.Clean(secretPath)

	// Check for path traversal attempts
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path cannot contain parent directory references")
	}

	// Ensure path doesn't start with /
	if strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("path should not start with /")
	}

	return nil
}
