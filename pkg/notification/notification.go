// Package notification defines the interface for notification providers
package notification

import (
	"context"
	"errors"
	"time"
)

// Notification types
type NotificationType string

const (
	TypeApprovalRequest  NotificationType = "approval_request"
	TypeApprovalReminder NotificationType = "approval_reminder"
	TypeApprovalResolved NotificationType = "approval_resolved"
	TypeSystemAlert      NotificationType = "system_alert"
)

// Priority levels
const (
	PriorityLow      int = 1
	PriorityMedium   int = 2
	PriorityHigh     int = 3
	PriorityCritical int = 4
)

// Action types
const (
	ActionApprove string = "approve"
	ActionDeny    string = "deny"
	ActionView    string = "view"
)

// Response types
const (
	ResponseApproved string = "approved"
	ResponseDenied   string = "denied"
	ResponseTimeout  string = "timeout"
)

// Common errors
var (
	ErrNotifierNotConfigured = errors.New("notifier not configured")
	ErrNotificationFailed    = errors.New("notification delivery failed")
	ErrInvalidPriority       = errors.New("invalid priority level")
	ErrNotifierNotFound      = errors.New("notifier not found")
)

// Notifier is the interface that all notification providers must implement
type Notifier interface {
	// Name returns the unique identifier for this notifier
	Name() string

	// Configure initializes the notifier with the given configuration
	Configure(config map[string]interface{}) error

	// Notify sends a notification
	Notify(ctx context.Context, notification Notification) error

	// Close cleans up any resources used by the notifier
	Close() error
}

// Notification represents a notification to be sent
type Notification struct {
	// ID is the unique identifier for this notification
	ID string `json:"id"`

	// Type is the notification type
	Type NotificationType `json:"type"`

	// Title is the notification title
	Title string `json:"title"`

	// Message is the notification body
	Message string `json:"message"`

	// Priority is the notification priority level (1-4)
	Priority int `json:"priority"`

	// Recipient is the target recipient (email, user ID, etc.)
	Recipient string `json:"recipient"`

	// Context contains additional context about the notification
	Context NotificationContext `json:"context,omitempty"`

	// Actions are available actions for interactive notifications
	Actions []Action `json:"actions,omitempty"`

	// CreatedAt is when the notification was created
	CreatedAt time.Time `json:"created_at"`
}

// NotificationContext contains context information for notifications
type NotificationContext struct {
	// RequestID is the associated approval request ID
	RequestID string `json:"request_id,omitempty"`

	// SecretPath is the path to the secret being requested
	SecretPath string `json:"secret_path,omitempty"`

	// RequestedBy is the user requesting access
	RequestedBy string `json:"requested_by,omitempty"`

	// RequestedAt is when the request was made
	RequestedAt time.Time `json:"requested_at,omitempty"`

	// ExpiresAt is when the request expires
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// Metadata contains additional provider-specific metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Action represents an available action in a notification
type Action struct {
	// Type is the action type (approve, deny, view)
	Type string `json:"type"`

	// Label is the display label for the action
	Label string `json:"label"`

	// URL is the callback URL for the action
	URL string `json:"url,omitempty"`

	// Value is the value to send when the action is triggered
	Value string `json:"value,omitempty"`
}

// ResponseCallback represents a response to an interactive notification
type ResponseCallback struct {
	// NotificationID is the ID of the notification being responded to
	NotificationID string `json:"notification_id"`

	// RequestID is the associated approval request ID
	RequestID string `json:"request_id"`

	// Response is the response type (approved, denied, timeout)
	Response string `json:"response"`

	// RespondedBy is the user who responded
	RespondedBy string `json:"responded_by"`

	// RespondedAt is when the response was received
	RespondedAt time.Time `json:"responded_at"`

	// Reason is an optional reason for the response
	Reason string `json:"reason,omitempty"`

	// Metadata contains additional response metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks if the notification is valid
func (n Notification) Validate() error {
	if n.ID == "" {
		return errors.New("notification ID is required")
	}
	if n.Type == "" {
		return errors.New("notification type is required")
	}
	if n.Recipient == "" {
		return errors.New("recipient is required")
	}
	if n.Priority < PriorityLow || n.Priority > PriorityCritical {
		return ErrInvalidPriority
	}
	return nil
}
