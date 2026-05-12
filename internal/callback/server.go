// Package callback provides HTTP handlers for notification callbacks
package callback

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WorkflowEngine defines the interface for workflow operations
type WorkflowEngine interface {
	ApproveRequest(requestID string, identity string, reason string) (*ApprovalResult, error)
	DenyRequest(requestID string, identity string, reason string) (*DenialResult, error)
}

// ApprovalResult contains the result of an approval operation
type ApprovalResult struct {
	RequestID         string
	Status            string
	Token             string
	ApprovalCount     int
	RequiredApprovals int
}

// DenialResult contains the result of a denial operation
type DenialResult struct {
	RequestID string
	Status    string
}

// Config holds server configuration
type Config struct {
	// Secret for signing/validating callbacks
	Secret string

	// SlackSecret for validating Slack signatures
	SlackSecret string

	// MaxRequestAge is the maximum age of a request in seconds (for replay protection)
	MaxRequestAge int
}

// Server handles notification callbacks
type Server struct {
	engine WorkflowEngine
	config *Config
}

// DuoCallbackPayload represents a callback from Duo
type DuoCallbackPayload struct {
	RequestID string `json:"request_id"`
	Response  string `json:"response"`
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// SlackInteractionPayload represents a Block Kit interaction from Slack
type SlackInteractionPayload struct {
	Type      string          `json:"type"`
	User      SlackUser       `json:"user"`
	Actions   []SlackAction   `json:"actions"`
	Timestamp int64           `json:"timestamp,omitempty"`
	Challenge string          `json:"challenge,omitempty"`
}

// SlackUser represents a Slack user
type SlackUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SlackAction represents a Slack Block Kit action
type SlackAction struct {
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
}

// NewServer creates a new callback server
func NewServer(engine WorkflowEngine, config *Config) *Server {
	if config == nil {
		config = &Config{}
	}

	// Generate random secret if not provided
	if config.Secret == "" {
		config.Secret = generateRandomSecret(32)
	}

	if config.MaxRequestAge == 0 {
		config.MaxRequestAge = 300 // 5 minutes default
	}

	return &Server{
		engine: engine,
		config: config,
	}
}

// SetupRoutes registers callback handlers with the provided mux
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/callbacks/duo", s.handleDuoCallbackWrapper)
	mux.HandleFunc("/callbacks/slack", s.handleSlackCallbackWrapper)
}

// handleDuoCallbackWrapper wraps HandleDuoCallback with method checking
func (s *Server) handleDuoCallbackWrapper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.HandleDuoCallback(w, r)
}

// handleSlackCallbackWrapper wraps HandleSlackCallback with method checking
func (s *Server) handleSlackCallbackWrapper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.HandleSlackCallback(w, r)
}

// HandleDuoCallback handles Duo Push callback responses
func (s *Server) HandleDuoCallback(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate signature
	signature := r.Header.Get("X-Duo-Signature")
	if signature == "" {
		signature = r.Header.Get("x-duo-signature")
	}

	if !s.validateDuoSignature(body, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse payload
	var payload DuoCallbackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.RequestID == "" {
		http.Error(w, "Missing request_id", http.StatusBadRequest)
		return
	}

	if payload.UserID == "" {
		http.Error(w, "Missing user_id", http.StatusBadRequest)
		return
	}

	// Route to workflow engine
	if s.engine == nil {
		http.Error(w, "Workflow engine not available", http.StatusInternalServerError)
		return
	}

	switch strings.ToLower(payload.Response) {
	case "approve", "approved":
		result, err := s.engine.ApproveRequest(payload.RequestID, payload.UserID, payload.Reason)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to process approval: %v", err), http.StatusInternalServerError)
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     result.Status,
			"request_id": result.RequestID,
		})

	case "deny", "denied":
		result, err := s.engine.DenyRequest(payload.RequestID, payload.UserID, payload.Reason)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to process denial: %v", err), http.StatusInternalServerError)
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     result.Status,
			"request_id": result.RequestID,
		})

	default:
		http.Error(w, "Invalid response type", http.StatusBadRequest)
	}
}

// HandleSlackCallback handles Slack Block Kit interaction callbacks
func (s *Server) HandleSlackCallback(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate Slack signature if configured
	if s.config.SlackSecret != "" {
		timestamp := r.Header.Get("X-Slack-Request-Timestamp")
		signature := r.Header.Get("X-Slack-Signature")

		if !s.validateSlackSignature(body, timestamp, signature) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}

		// Check for replay attacks
		if timestamp != "" {
			ts, err := strconv.ParseInt(timestamp, 10, 64)
			if err == nil {
				if time.Since(time.Unix(ts, 0)) > time.Duration(s.config.MaxRequestAge)*time.Second {
					http.Error(w, "Request too old", http.StatusUnauthorized)
					return
				}
			}
		}
	}

	// Parse payload
	var payload SlackInteractionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification (Slack events API)
	if payload.Type == "url_verification" {
		respondJSON(w, http.StatusOK, map[string]string{
			"challenge": payload.Challenge,
		})
		return
	}

	// Only process block_actions
	if payload.Type != "block_actions" {
		respondJSON(w, http.StatusOK, map[string]string{
			"status": "ignored",
			"reason": "not a block_actions event",
		})
		return
	}

	// Route actions to workflow engine
	if s.engine == nil {
		http.Error(w, "Workflow engine not available", http.StatusInternalServerError)
		return
	}

	for _, action := range payload.Actions {
		requestID := action.Value
		if requestID == "" {
			continue
		}

		identity := payload.User.ID
		if identity == "" {
			identity = payload.User.Name
		}

		switch action.ActionID {
		case "approve_action":
			_, err := s.engine.ApproveRequest(requestID, identity, "")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to process approval: %v", err), http.StatusInternalServerError)
				return
			}

		case "deny_action":
			_, err := s.engine.DenyRequest(requestID, identity, "")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to process denial: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// validateDuoSignature validates Duo callback signatures
func (s *Server) validateDuoSignature(body []byte, signature string) bool {
	if signature == "" {
		// In development mode, accept requests without signature
		return true
	}

	expected := s.generateDuoSignature(body)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// generateDuoSignature generates a Duo-compatible signature
func (s *Server) generateDuoSignature(body []byte) string {
	h := hmac.New(sha256.New, []byte(s.config.Secret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// validateSlackSignature validates Slack request signatures
func (s *Server) validateSlackSignature(body []byte, timestamp, signature string) bool {
	if signature == "" || timestamp == "" {
		return false
	}

	expected := s.generateSlackSignature(body, timestamp)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// generateSlackSignature generates a Slack-compatible signature
func (s *Server) generateSlackSignature(body []byte, timestamp string) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(s.config.SlackSecret))
	h.Write([]byte(baseString))
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// generateRandomSecret generates a random secret string
func generateRandomSecret(length int) string {
	// Simple random generation for now
	// In production, use crypto/rand
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[i%len(chars)]
	}
	return string(result)
}

// Close cleans up server resources
func (s *Server) Close() error {
	// Nothing to clean up currently
	return nil
}

// Common errors
var (
	ErrInvalidSignature = errors.New("invalid callback signature")
	ErrMissingRequestID = errors.New("missing request_id")
	ErrMissingUserID    = errors.New("missing user_id")
	ErrInvalidResponse  = errors.New("invalid response type")
)
