// Package client provides a Go client library for the Traverse API.
// It allows programmatic interaction with the Traverse secret management system.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client provides methods to interact with the Traverse API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// CreateRequestRequest represents a request to create a new secret request.
type CreateRequestRequest struct {
	SecretPath              string            `json:"secret_path"`
	Reason                  string            `json:"reason"`
	ClientID                string            `json:"client_id,omitempty"`
	RequestedDuration       string            `json:"requested_duration,omitempty"`
	NotificationPreferences []string          `json:"notification_preferences,omitempty"`
	Metadata                map[string]string `json:"metadata,omitempty"`
}

// CreateRequestResponse represents the response from creating a request.
type CreateRequestResponse struct {
	RequestID             string    `json:"request_id"`
	Status                string    `json:"status"`
	Message               string    `json:"message"`
	PollURL               string    `json:"poll_url"`
	WebSocketURL          string    `json:"websocket_url"`
	ExpiresAt             time.Time `json:"expires_at"`
	EstimatedApprovalTime string    `json:"estimated_approval_time"`
}

// GetStatusResponse represents the status of a secret request.
type GetStatusResponse struct {
	RequestID         string             `json:"request_id"`
	Status            string             `json:"status"`
	StatusDetail      string             `json:"status_detail"`
	ClientID          string             `json:"client_id"`
	SecretPath        string             `json:"secret_path"`
	Reason            string             `json:"reason"`
	CreatedAt         time.Time          `json:"created_at"`
	ExpiresAt         time.Time          `json:"expires_at"`
	ApproversNotified []ApproverNotified `json:"approvers_notified,omitempty"`
	ApprovalCount     int                `json:"approval_count"`
	RequiredApprovals int                `json:"required_approvals"`
	DenialCount       int                `json:"denial_count"`
	ApprovedAt        *time.Time         `json:"approved_at,omitempty"`
	ApprovedBy        []ApprovalInfo     `json:"approved_by,omitempty"`
	Token             string             `json:"token,omitempty"`
	TokenExpiresAt    *time.Time         `json:"token_expires_at,omitempty"`
	SecretURL         string             `json:"secret_url,omitempty"`
	DeniedAt          *time.Time         `json:"denied_at,omitempty"`
	DeniedBy          []DenialInfo       `json:"denied_by,omitempty"`
	Message           string             `json:"message,omitempty"`
}

// ApproverNotified represents an approver who was notified.
type ApproverNotified struct {
	Identity   string    `json:"identity"`
	NotifiedAt time.Time `json:"notified_at"`
	Channel    string    `json:"channel"`
}

// ApprovalInfo represents approval details.
type ApprovalInfo struct {
	Identity   string    `json:"identity"`
	ApprovedAt time.Time `json:"approved_at"`
	Reason     string    `json:"reason"`
}

// DenialInfo represents denial details.
type DenialInfo struct {
	Identity string    `json:"identity"`
	DeniedAt time.Time `json:"denied_at"`
	Reason   string    `json:"reason"`
}

// ApproveRequestRequest represents an approval request.
type ApproveRequestRequest struct {
	Reason           string   `json:"reason,omitempty"`
	OverrideDuration string   `json:"override_duration,omitempty"`
	ApprovedPaths    []string `json:"approved_paths,omitempty"`
}

// ApproveRequestResponse represents the response from approving a request.
type ApproveRequestResponse struct {
	RequestID                  string    `json:"request_id"`
	Status                     string    `json:"status"`
	Message                    string    `json:"message"`
	ApprovedAt                 time.Time `json:"approved_at"`
	RemainingRequiredApprovals int       `json:"remaining_required_approvals"`
}

// DenyRequestRequest represents a denial request.
type DenyRequestRequest struct {
	Reason string `json:"reason"`
}

// DenyRequestResponse represents the response from denying a request.
type DenyRequestResponse struct {
	RequestID string    `json:"request_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	DeniedAt  time.Time `json:"denied_at"`
}

// SecretResponse represents a secret value response.
type SecretResponse struct {
	Path     string                 `json:"path"`
	Provider string                 `json:"provider"`
	Values   map[string]string      `json:"values"`
	Metadata map[string]interface{} `json:"metadata"`
	Access   AccessInfo             `json:"access"`
}

// AccessInfo contains access metadata for a secret.
type AccessInfo struct {
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at"`
	RequestID string    `json:"request_id"`
}

// APIError represents an error response from the API.
type APIError struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (status %d): %s", e.Title, e.Status, e.Detail)
}

// NewClient creates a new Traverse API client.
// baseURL is the URL of the Traverse server (e.g., "http://localhost:8080").
// apiKey is optional and can be empty.
func NewClient(baseURL, apiKey string) (*Client, error) {
	// Remove trailing slash from baseURL
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// CreateRequest submits a new secret request.
func (c *Client) CreateRequest(req CreateRequestRequest) (*CreateRequestResponse, error) {
	url := fmt.Sprintf("%s/v1/requests", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, c.handleErrorResponse(resp)
	}

	var result CreateRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetStatus retrieves the status of a secret request.
func (c *Client) GetStatus(requestID string) (*GetStatusResponse, error) {
	url := fmt.Sprintf("%s/v1/requests/%s/status", c.baseURL, requestID)

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result GetStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ApproveRequest approves a pending secret request.
func (c *Client) ApproveRequest(requestID string, req ApproveRequestRequest) (*ApproveRequestResponse, error) {
	url := fmt.Sprintf("%s/v1/requests/%s/approve", c.baseURL, requestID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result ApproveRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DenyRequest denies a pending secret request.
func (c *Client) DenyRequest(requestID string, req DenyRequestRequest) (*DenyRequestResponse, error) {
	url := fmt.Sprintf("%s/v1/requests/%s/deny", c.baseURL, requestID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result DenyRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSecret retrieves a secret value using an access token.
func (c *Client) GetSecret(path, token string) (*SecretResponse, error) {
	url := fmt.Sprintf("%s/v1/secrets/%s?token=%s", c.baseURL, path, token)

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result SecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PollStatus polls the status of a request until it's no longer pending or maxAttempts is reached.
// interval is the time to wait between polls.
// Returns the final status response or an error if max attempts exceeded.
func (c *Client) PollStatus(requestID string, interval time.Duration, maxAttempts int) (*GetStatusResponse, error) {
	for i := 0; i < maxAttempts; i++ {
		resp, err := c.GetStatus(requestID)
		if err != nil {
			return nil, err
		}

		if resp.Status != "pending" {
			return resp, nil
		}

		if i < maxAttempts-1 {
			time.Sleep(interval)
		}
	}

	return nil, fmt.Errorf("polling timed out after %d attempts", maxAttempts)
}

// handleErrorResponse parses an error response from the API.
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return &apiErr
}
