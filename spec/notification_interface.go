// Package notification defines the interface for notification providers
package notification

import (
	"context"
	"time"
)

// Provider is the interface that all notification providers must implement
type Provider interface {
	// Name returns the unique identifier for this provider
	Name() string

	// Configure initializes the provider with the given configuration
	Configure(config map[string]interface{}) error

	// Send sends a notification to the approver
	Send(ctx context.Context, notification *Notification) (*Result, error)

	// SupportsInteractive returns true if this provider supports interactive approval
	// (e.g., Duo push with approve/deny buttons)
	SupportsInteractive() bool

	// Health checks the provider's connectivity
	Health(ctx context.Context) error
}

// Notification contains the information needed to notify an approver
type Notification struct {
	// RequestID is the unique identifier for this request
	RequestID string

	// ClientID identifies who is requesting access
	ClientID string

	// SecretPath is the path to the requested secret
	SecretPath string

	// Reason explains why access is needed
	Reason string

	// RequestedAt is when the request was created
	RequestedAt time.Time

	// ExpiresAt is when the request expires
	ExpiresAt time.Time

	// Approvers is the list of identities who can approve
	Approvers []Approver

	// Metadata contains additional request context
	Metadata map[string]string
}

// Approver represents someone who can approve a request
type Approver struct {
	// Identity is the unique identifier (email, username, etc.)
	Identity string

	// Name is the display name
	Name string

	// ContactInfo is provider-specific contact info
	// For Duo: user_id
	// For Slack: user_id
	// For Email: email address
	ContactInfo string

	// Channels lists which notification channels to use
	Channels []string
}

// Result contains information about the sent notification
type Result struct {
	// Success indicates if notification was sent
	Success bool

	// Provider is the name of the notification provider used
	Provider string

	// ExternalID is the provider's message/notification ID
	ExternalID string

	// SentAt is when the notification was sent
	SentAt time.Time

	// Error contains error details if Success is false
	Error string

	// InteractiveURL is a URL for interactive approval (if supported)
	InteractiveURL string
}

// CallbackHandler handles interactive approval callbacks
type CallbackHandler interface {
	// HandleApproval processes an approval callback
	HandleApproval(ctx context.Context, requestID string, approver Approver, reason string) error

	// HandleDenial processes a denial callback  
	HandleDenial(ctx context.Context, requestID string, approver Approver, reason string) error
}

// Duo Provider
// Sends push notifications via Duo Security
// Supports interactive approve/deny buttons

type DuoConfig struct {
	IntegrationKey string
	SecretKey      string
	APIHostname    string
}

// Pushover Provider
// Sends push notifications via Pushover
// Includes URL action buttons

type PushoverConfig struct {
	AppToken string
	UserKey  string
	Priority int // -2 to 2, 2 = emergency (requires acknowledgement)
}

// Slack Provider
// Sends DMs or channel messages via Slack
// Can include interactive blocks with approve/deny buttons

type SlackConfig struct {
	BotToken       string
	ApproverChannel string // Channel for notifications
	DMUsers        bool    // Send DMs to approvers
}

// Telegram Provider
// Sends messages via Telegram Bot API
// Can include inline keyboards for approval

type TelegramConfig struct {
	BotToken string
	ChatIDs  []int64 // Pre-approved chat IDs
}

// Webhook Provider
// POSTs to custom webhook URL
// Expects webhook to call back to Sentinel API

type WebhookConfig struct {
	URL     string
	Headers map[string]string
	// Secret for HMAC signature
	Secret string
}

// Email Provider
// Sends emails via SMTP
// Contains links to web UI for approval

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	UseTLS       bool
}

// Notification priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// NotificationTemplate defines the message template
type NotificationTemplate struct {
	Subject string
	Body    string
	// Action buttons for interactive notifications
	Actions struct {
		ApproveText string
		DenyText    string
		ViewText    string
	}
}

// Default templates
templates := map[Priority]NotificationTemplate{
	PriorityNormal: {
		Subject: "Secret Access Request: {{.SecretPath}}",
		Body: `
Client: {{.ClientID}}
Path: {{.SecretPath}}
Reason: {{.Reason}}
Expires: {{.ExpiresAt}}

Approve: {{.ApproveURL}}
Deny: {{.DenyURL}}
`,
		Actions: struct {
			ApproveText string
			DenyText    string
			ViewText    string
		}{
			ApproveText: "✅ Approve",
			DenyText:    "❌ Deny",
			ViewText:    "👁️ View Details",
		},
	},
	PriorityCritical: {
		Subject: "🚨 URGENT: Secret Access Request",
		Body: `
CRITICAL: Client {{.ClientID}} is requesting access to {{.SecretPath}}

Reason: {{.Reason}}
Request Time: {{.RequestedAt}}
EXPIRES IN: {{.TimeUntilExpiry}}

This request requires immediate attention.

Approve: {{.ApproveURL}}
Deny: {{.DenyURL}}
`,
		Actions: struct {
			ApproveText string
			DenyText    string
			ViewText    string
		}{
			ApproveText: "✅ APPROVE NOW",
			DenyText:    "❌ DENY",
			ViewText:    "Details",
		},
	},
}

// NotificationManager manages multiple notification providers
type Manager struct {
	providers map[string]Provider
	templates map[Priority]NotificationTemplate
}

// NewManager creates a new notification manager
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		templates: templates,
	}
}

// Register adds a notification provider
func (m *Manager) Register(name string, provider Provider) {
	m.providers[name] = provider
}

// SendNotification sends notification via all configured providers for the approver
func (m *Manager) SendNotification(ctx context.Context, notification *Notification) ([]*Result, error) {
	var results []*Result
	
	for _, approver := range notification.Approvers {
		for _, channel := range approver.Channels {
			provider, ok := m.providers[channel]
			if !ok {
				results = append(results, &Result{
					Success: false,
					Provider: channel,
					Error: "provider not configured",
				})
				continue
			}
			
			result, err := provider.Send(ctx, notification)
			if err != nil {
				result = &Result{
					Success: false,
					Provider: channel,
					Error: err.Error(),
				}
			}
			results = append(results, result)
		}
	}
	
	return results, nil
}

// Duo provider implementation
// POST /auth/v2/preauth to check if user exists
// POST /auth/v2/auth to send push notification
// Poll /auth/v2/auth_status for response

type DuoProvider struct {
	config DuoConfig
	client *http.Client
}

func (d *DuoProvider) Send(ctx context.Context, n *Notification) (*Result, error) {
	// 1. Check if user exists in Duo
	// 2. Send push notification with details
	// 3. Return tracking ID for polling
	// Note: Duo requires separate polling for response
	
	return &Result{
		Success:    true,
		Provider:   "duo",
		ExternalID: "duo_txn_id",
		SentAt:     time.Now(),
	}, nil
}

func (d *DuoProvider) SupportsInteractive() bool {
	return true
}

// Slack provider implementation
// Uses chat.postMessage API
// Can include blocks with interactive elements

type SlackProvider struct {
	config SlackConfig
	client *http.Client
}

func (s *SlackProvider) Send(ctx context.Context, n *Notification) (*Result, error) {
	// 1. Format message with blocks
	// 2. Post to approver's DM or channel
	// 3. Include interactive buttons with callback IDs
	
	return &Result{
		Success:    true,
		Provider:   "slack",
		ExternalID: "slack_ts",
		SentAt:     time.Now(),
		InteractiveURL: "", // Not needed, uses interactive blocks
	}, nil
}

func (s *SlackProvider) SupportsInteractive() bool {
	return true
}

// Callback server for interactive notifications
// Receives webhooks from Duo, Slack, etc.

type CallbackServer struct {
	handler CallbackHandler
}

func (s *CallbackServer) HandleDuoCallback(w http.ResponseWriter, r *http.Request) {
	// Parse Duo authentication response
	// Call handler.HandleApproval or HandleDenial
}

func (s *CallbackServer) HandleSlackCallback(w http.ResponseWriter, r *http.Request) {
	// Parse Slack interaction payload
	// Extract request_id from callback_id
	// Call handler.HandleApproval or HandleDenial
}
