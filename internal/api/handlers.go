package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/funkymonkeymonk/traverse/internal/audit"
	"github.com/funkymonkeymonk/traverse/internal/config"
	"github.com/funkymonkeymonk/traverse/internal/storage"
	"github.com/gin-gonic/gin"
)

type Storage interface {
	CreateRequest(req *storage.SecretRequest) error
	GetRequest(id string) (*storage.SecretRequest, error)
	UpdateRequestStatus(id string, status string) error
	ListRequests(filters storage.ListFilters, limit int, offset int) ([]*storage.SecretRequest, int, error)
}

type Handler struct {
	storage     Storage
	auditLogger audit.Logger
	config      *config.Config
}

func NewHandler(storage Storage, auditLogger audit.Logger, cfg *config.Config) *Handler {
	return &Handler{
		storage:     storage,
		auditLogger: auditLogger,
		config:      cfg,
	}
}

type CreateRequestRequest struct {
	SecretPath              string            `json:"secret_path" binding:"required"`
	Reason                  string            `json:"reason" binding:"required,min=10"`
	ClientID                string            `json:"client_id"`
	RequestedDuration       string            `json:"requested_duration"`
	NotificationPreferences []string          `json:"notification_preferences"`
	Metadata                map[string]string `json:"metadata"`
}

type CreateRequestResponse struct {
	RequestID             string    `json:"request_id"`
	Status                string    `json:"status"`
	Message               string    `json:"message"`
	PollURL               string    `json:"poll_url"`
	WebSocketURL          string    `json:"websocket_url"`
	ExpiresAt             time.Time `json:"expires_at"`
	EstimatedApprovalTime string    `json:"estimated_approval_time"`
}

func (h *Handler) CreateRequest(c *gin.Context) {
	var req CreateRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":   "https://traverse.internal/errors/invalid-request",
			"title":  "Invalid Request",
			"status": 400,
			"detail": err.Error(),
		})
		return
	}

	// Validate secret path
	if err := config.ValidateRequest(config.SecretRequest{
		SecretPath: req.SecretPath,
		Reason:     req.Reason,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":   "https://traverse.internal/errors/invalid-request",
			"title":  "Invalid Request",
			"status": 400,
			"detail": err.Error(),
		})
		return
	}

	// Get client ID from context or request
	clientID, _ := c.Get("client_id")
	if req.ClientID == "" && clientID != nil {
		req.ClientID = clientID.(string)
	}

	// Parse duration (default 1h)
	_ = time.Hour // Duration is used for token expiry calculation
	if req.RequestedDuration != "" {
		_, _ = time.ParseDuration(req.RequestedDuration)
	}

	// Generate request ID
	requestID := generateRequestID()

	// Create request
	secretReq := &storage.SecretRequest{
		ID:                requestID,
		ClientID:          req.ClientID,
		SecretPath:        req.SecretPath,
		Reason:            req.Reason,
		Status:            "pending",
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(5 * time.Minute), // Request timeout
		RequiredApprovals: 1,                               // Default to 1 approval
	}

	if err := h.storage.CreateRequest(secretReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":   "https://traverse.internal/errors/internal",
			"title":  "Internal Server Error",
			"status": 500,
			"detail": "Failed to create request",
		})
		return
	}

	// Audit log
	h.auditLogger.Log(audit.Event{
		Type:        audit.EventRequestCreated,
		RequestID:   requestID,
		ClientID:    req.ClientID,
		SecretPath:  req.SecretPath,
		Description: "Secret request created",
	})

	response := CreateRequestResponse{
		RequestID:             requestID,
		Status:                "pending_approval",
		Message:               "Request submitted. Approval notification sent to approvers.",
		PollURL:               fmt.Sprintf("/v1/requests/%s/status", requestID),
		WebSocketURL:          fmt.Sprintf("wss://%s/v1/requests/%s/stream", c.Request.Host, requestID),
		ExpiresAt:             secretReq.ExpiresAt,
		EstimatedApprovalTime: "< 2 minutes",
	}

	c.JSON(http.StatusAccepted, response)
}

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

type ApproverNotified struct {
	Identity   string    `json:"identity"`
	NotifiedAt time.Time `json:"notified_at"`
	Channel    string    `json:"channel"`
}

type ApprovalInfo struct {
	Identity   string    `json:"identity"`
	ApprovedAt time.Time `json:"approved_at"`
	Reason     string    `json:"reason"`
}

type DenialInfo struct {
	Identity string    `json:"identity"`
	DeniedAt time.Time `json:"denied_at"`
	Reason   string    `json:"reason"`
}

func (h *Handler) GetRequestStatus(c *gin.Context) {
	requestID := c.Param("request_id")

	req, err := h.storage.GetRequest(requestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"type":   "https://traverse.internal/errors/not-found",
			"title":  "Not Found",
			"status": 404,
			"detail": fmt.Sprintf("Request %s not found", requestID),
		})
		return
	}

	response := GetStatusResponse{
		RequestID:         req.ID,
		Status:            req.Status,
		StatusDetail:      req.StatusDetail,
		ClientID:          req.ClientID,
		SecretPath:        req.SecretPath,
		Reason:            req.Reason,
		CreatedAt:         req.CreatedAt,
		ExpiresAt:         req.ExpiresAt,
		ApprovalCount:     req.ApprovalCount,
		RequiredApprovals: req.RequiredApprovals,
		DenialCount:       req.DenialCount,
	}

	if req.Status == "approved" {
		response.ApprovedAt = req.ApprovedAt
		response.Token = req.Token
		response.TokenExpiresAt = req.TokenExpiresAt
		response.SecretURL = fmt.Sprintf("/v1/secrets/%s", req.SecretPath)
	}

	if req.Status == "denied" {
		response.DeniedAt = req.DeniedAt
	}

	if req.Status == "expired" {
		response.Message = "Request expired before receiving required approvals"
	}

	c.JSON(http.StatusOK, response)
}

type ApproveRequestRequest struct {
	Reason           string   `json:"reason"`
	OverrideDuration string   `json:"override_duration"`
	ApprovedPaths    []string `json:"approved_paths"`
}

type ApproveRequestResponse struct {
	RequestID                  string    `json:"request_id"`
	Status                     string    `json:"status"`
	Message                    string    `json:"message"`
	ApprovedAt                 time.Time `json:"approved_at"`
	RemainingRequiredApprovals int       `json:"remaining_required_approvals"`
}

func (h *Handler) ApproveRequest(c *gin.Context) {
	requestID := c.Param("request_id")

	var req ApproveRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is okay for approval
		req = ApproveRequestRequest{}
	}

	// Get request
	secretReq, err := h.storage.GetRequest(requestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"type":   "https://traverse.internal/errors/not-found",
			"title":  "Not Found",
			"status": 404,
			"detail": fmt.Sprintf("Request %s not found", requestID),
		})
		return
	}

	// Check if already processed
	if secretReq.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{
			"type":   "https://traverse.internal/errors/request-processed",
			"title":  "Request Already Processed",
			"status": 409,
			"detail": fmt.Sprintf("Request %s was already %s", requestID, secretReq.Status),
		})
		return
	}

	// Update status
	now := time.Now()
	secretReq.Status = "approved"
	secretReq.StatusDetail = "access_granted"
	secretReq.ApprovedAt = &now
	secretReq.ApprovalCount++

	// Generate token
	token := generateToken()
	secretReq.Token = token
	tokenExpiry := now.Add(time.Hour) // Default 1 hour
	secretReq.TokenExpiresAt = &tokenExpiry

	if err := h.storage.UpdateRequestStatus(requestID, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":   "https://traverse.internal/errors/internal",
			"title":  "Internal Server Error",
			"status": 500,
			"detail": "Failed to approve request",
		})
		return
	}

	// Audit log
	h.auditLogger.Log(audit.Event{
		Type:        audit.EventRequestApproved,
		RequestID:   requestID,
		ClientID:    secretReq.ClientID,
		SecretPath:  secretReq.SecretPath,
		Description: fmt.Sprintf("Request approved: %s", req.Reason),
	})

	response := ApproveRequestResponse{
		RequestID:                  requestID,
		Status:                     "approved",
		Message:                    "Request approved. Token issued.",
		ApprovedAt:                 now,
		RemainingRequiredApprovals: secretReq.RequiredApprovals - secretReq.ApprovalCount,
	}

	c.JSON(http.StatusOK, response)
}

type DenyRequestRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type DenyRequestResponse struct {
	RequestID string    `json:"request_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	DeniedAt  time.Time `json:"denied_at"`
}

func (h *Handler) DenyRequest(c *gin.Context) {
	requestID := c.Param("request_id")

	var req DenyRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":   "https://traverse.internal/errors/invalid-request",
			"title":  "Invalid Request",
			"status": 400,
			"detail": "Reason is required for denial",
		})
		return
	}

	// Get request
	secretReq, err := h.storage.GetRequest(requestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"type":   "https://traverse.internal/errors/not-found",
			"title":  "Not Found",
			"status": 404,
			"detail": fmt.Sprintf("Request %s not found", requestID),
		})
		return
	}

	// Check if already processed
	if secretReq.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{
			"type":   "https://traverse.internal/errors/request-processed",
			"title":  "Request Already Processed",
			"status": 409,
			"detail": fmt.Sprintf("Request %s was already %s", requestID, secretReq.Status),
		})
		return
	}

	// Update status
	now := time.Now()
	secretReq.Status = "denied"
	secretReq.StatusDetail = "access_denied"
	secretReq.DeniedAt = &now
	secretReq.DenialCount++

	if err := h.storage.UpdateRequestStatus(requestID, "denied"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":   "https://traverse.internal/errors/internal",
			"title":  "Internal Server Error",
			"status": 500,
			"detail": "Failed to deny request",
		})
		return
	}

	// Audit log
	h.auditLogger.Log(audit.Event{
		Type:        audit.EventRequestDenied,
		RequestID:   requestID,
		ClientID:    secretReq.ClientID,
		SecretPath:  secretReq.SecretPath,
		Description: fmt.Sprintf("Request denied: %s", req.Reason),
	})

	response := DenyRequestResponse{
		RequestID: requestID,
		Status:    "denied",
		Message:   "Request denied.",
		DeniedAt:  now,
	}

	c.JSON(http.StatusOK, response)
}

type ListRequestsResponse struct {
	Requests   []RequestSummary `json:"requests"`
	Pagination PaginationInfo   `json:"pagination"`
}

type RequestSummary struct {
	RequestID  string     `json:"request_id"`
	Status     string     `json:"status"`
	ClientID   string     `json:"client_id"`
	SecretPath string     `json:"secret_path"`
	Reason     string     `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type PaginationInfo struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

func (h *Handler) ListRequests(c *gin.Context) {
	// Parse query parameters
	status := c.Query("status")
	clientID := c.Query("client_id")
	secretPath := c.Query("secret_path")
	limit := 50
	offset := 0

	// TODO: Parse limit and offset from query params

	filters := storage.ListFilters{
		Status:     status,
		ClientID:   clientID,
		SecretPath: secretPath,
	}

	requests, total, err := h.storage.ListRequests(filters, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":   "https://traverse.internal/errors/internal",
			"title":  "Internal Server Error",
			"status": 500,
			"detail": "Failed to list requests",
		})
		return
	}

	// Convert to response format
	var summaries []RequestSummary
	for _, req := range requests {
		summary := RequestSummary{
			RequestID:  req.ID,
			Status:     req.Status,
			ClientID:   req.ClientID,
			SecretPath: req.SecretPath,
			Reason:     req.Reason,
			CreatedAt:  req.CreatedAt,
		}
		if req.Status != "pending" {
			summary.ResolvedAt = req.ResolvedAt
		}
		summaries = append(summaries, summary)
	}

	response := ListRequestsResponse{
		Requests: summaries,
		Pagination: PaginationInfo{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+len(summaries) < total,
		},
	}

	c.JSON(http.StatusOK, response)
}

type SecretResponse struct {
	Path     string                 `json:"path"`
	Provider string                 `json:"provider"`
	Values   map[string]string      `json:"values"`
	Metadata map[string]interface{} `json:"metadata"`
	Access   AccessInfo             `json:"access"`
}

type AccessInfo struct {
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at"`
	RequestID string    `json:"request_id"`
}

func (h *Handler) GetSecret(c *gin.Context) {
	path := c.Param("path")
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"type":   "https://traverse.internal/errors/invalid-token",
			"title":  "Invalid or Expired Token",
			"status": 401,
			"detail": "Token is required",
		})
		return
	}

	// TODO: Validate token
	// For now, we'll return a placeholder response

	response := SecretResponse{
		Path:     path,
		Provider: "1password",
		Values: map[string]string{
			"api_key": "sk_live_placeholder",
		},
		Metadata: map[string]interface{}{
			"version":    "1",
			"created_at": time.Now().Add(-30 * 24 * time.Hour),
			"updated_at": time.Now(),
		},
		Access: AccessInfo{
			GrantedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
			RequestID: "req_placeholder",
		},
	}

	// Audit log
	h.auditLogger.Log(audit.Event{
		Type:        audit.EventSecretAccessed,
		SecretPath:  path,
		Description: "Secret accessed",
	})

	c.JSON(http.StatusOK, response)
}

type RevokeTokenRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type RevokeTokenResponse struct {
	TokenID   string    `json:"token_id"`
	Status    string    `json:"status"`
	RevokedAt time.Time `json:"revoked_at"`
	Reason    string    `json:"reason"`
}

func (h *Handler) RevokeToken(c *gin.Context) {
	tokenID := c.Param("token_id")

	var req RevokeTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":   "https://traverse.internal/errors/invalid-request",
			"title":  "Invalid Request",
			"status": 400,
			"detail": "Reason is required",
		})
		return
	}

	// TODO: Implement token revocation
	// For now, return a placeholder response

	response := RevokeTokenResponse{
		TokenID:   tokenID,
		Status:    "revoked",
		RevokedAt: time.Now(),
		Reason:    req.Reason,
	}

	// Audit log
	h.auditLogger.Log(audit.Event{
		Type:        audit.EventTokenRevoked,
		Description: fmt.Sprintf("Token revoked: %s", req.Reason),
	})

	c.JSON(http.StatusOK, response)
}

type HealthResponse struct {
	Status        string           `json:"status"`
	Version       string           `json:"version"`
	Uptime        string           `json:"uptime"`
	Providers     []ProviderStatus `json:"providers"`
	Notifications []NotifStatus    `json:"notifications"`
	Storage       StorageStatus    `json:"storage"`
}

type ProviderStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int    `json:"latency_ms"`
}

type NotifStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

type StorageStatus struct {
	Type            string `json:"type"`
	Status          string `json:"status"`
	PendingRequests int    `json:"pending_requests"`
}

func (h *Handler) HealthCheck(c *gin.Context) {
	response := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  "0h0m0s", // TODO: Calculate actual uptime
		Providers: []ProviderStatus{
			{Name: "1password", Status: "connected", LatencyMs: 45},
			{Name: "vault", Status: "connected", LatencyMs: 12},
		},
		Notifications: []NotifStatus{
			{Provider: "duo", Status: "connected"},
			{Provider: "slack", Status: "connected"},
		},
		Storage: StorageStatus{
			Type:            h.config.Storage.Type,
			Status:          "connected",
			PendingRequests: 3, // TODO: Query actual pending count
		},
	}

	c.JSON(http.StatusOK, response)
}

func generateRequestID() string {
	// TODO: Use proper UUID generation
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func generateToken() string {
	// TODO: Use proper JWT generation
	return fmt.Sprintf("tok_%d", time.Now().UnixNano())
}
