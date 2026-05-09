// Package slack implements a Slack notification provider using Block Kit
package slack

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/notification"
	"github.com/slack-go/slack"
)

const (
	providerName      = "slack"
	defaultTimeout    = 30
	defaultMaxRetries = 3
	defaultRateLimit  = 20 // requests per minute
)

// Common errors
var (
	ErrNotConfigured      = errors.New("slack provider not configured")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrSlackAPIError      = errors.New("slack api error")
	ErrMissingBotToken    = errors.New("bot_token is required")
	ErrMissingChannel     = errors.New("channel is required")
)

// Config holds Slack provider configuration
type Config struct {
	// BotToken is the Slack bot token (xoxb-...)
	BotToken string

	// Channel is the default channel to post to
	Channel string

	// Timeout for API calls in seconds
	Timeout int

	// MaxRetries for failed API calls
	MaxRetries int

	// RateLimitPerMinute limits how many notifications per minute
	RateLimitPerMinute int

	// WebhookURL for incoming webhooks (optional, alternative to bot token)
	WebhookURL string
}

// Provider implements the notification.Notifier interface for Slack
type Provider struct {
	config      *Config
	client      *slack.Client
	rateLimiter *RateLimiter
	configured  bool
	mu          sync.RWMutex
}

// RateLimiter implements a simple rate limiter
type RateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	interval := time.Minute / time.Duration(requestsPerMinute)
	rl := &RateLimiter{
		tokens:   make(chan struct{}, requestsPerMinute),
		interval: interval,
	}

	// Fill the bucket
	for i := 0; i < requestsPerMinute; i++ {
		rl.tokens <- struct{}{}
	}

	// Start token refill goroutine
	go rl.refill()

	return rl
}

// refill adds tokens at the specified interval
func (rl *RateLimiter) refill() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full, skip
		}
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// NewProvider creates a new Slack provider instance
func NewProvider() *Provider {
	return &Provider{
		configured: false,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return providerName
}

// Configure initializes the provider with the given configuration
func (p *Provider) Configure(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &Config{}

	// Parse bot_token
	botToken, ok := config["bot_token"].(string)
	if !ok || botToken == "" {
		// Check if webhook_url is provided as alternative
		webhookURL, webhookOk := config["webhook_url"].(string)
		if !webhookOk || webhookURL == "" {
			return ErrMissingBotToken
		}
		cfg.WebhookURL = webhookURL
	} else {
		cfg.BotToken = botToken
	}

	// Parse channel
	channel, ok := config["channel"].(string)
	if !ok || channel == "" {
		return ErrMissingChannel
	}
	cfg.Channel = channel

	// Parse timeout with default
	cfg.Timeout = defaultTimeout
	if timeout, ok := config["timeout"].(int); ok && timeout > 0 {
		cfg.Timeout = timeout
	}

	// Parse max_retries with default
	cfg.MaxRetries = defaultMaxRetries
	if maxRetries, ok := config["max_retries"].(int); ok && maxRetries > 0 {
		cfg.MaxRetries = maxRetries
	}

	// Parse rate limit with default
	cfg.RateLimitPerMinute = defaultRateLimit
	if rateLimit, ok := config["rate_limit_per_minute"].(int); ok && rateLimit > 0 {
		cfg.RateLimitPerMinute = rateLimit
	}

	// Initialize Slack client if using bot token
	if cfg.BotToken != "" {
		p.client = slack.New(cfg.BotToken)
	}

	p.config = cfg
	p.rateLimiter = NewRateLimiter(cfg.RateLimitPerMinute)
	p.configured = true

	return nil
}

// Notify sends a notification via Slack
func (p *Provider) Notify(ctx context.Context, notif notification.Notification) error {
	p.mu.RLock()
	if !p.configured {
		p.mu.RUnlock()
		return ErrNotConfigured
	}
	rateLimiter := p.rateLimiter
	client := p.client
	config := p.config
	p.mu.RUnlock()

	// Check rate limit
	if !rateLimiter.Allow() {
		return ErrRateLimitExceeded
	}

	// Build message blocks based on notification type
	var blocks []slack.Block
	switch notif.Type {
	case notification.TypeApprovalRequest:
		blocks = buildApprovalBlocks(notif)
	case notification.TypeSystemAlert:
		blocks = buildAlertBlocks(notif)
	default:
		blocks = buildDefaultBlocks(notif)
	}

	// Send the message
	if config.WebhookURL != "" {
		return p.sendViaWebhook(ctx, config.WebhookURL, notif, blocks)
	}

	return p.sendViaAPI(ctx, client, config.Channel, notif, blocks)
}

// sendViaAPI sends message using Slack API
func (p *Provider) sendViaAPI(ctx context.Context, client *slack.Client, channel string, notif notification.Notification, blocks []slack.Block) error {
	if client == nil {
		return fmt.Errorf("%w: slack client not initialized", ErrSlackAPIError)
	}

	_, _, err := client.PostMessageContext(
		ctx,
		channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(notif.Message, false),
	)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrSlackAPIError, err)
	}

	return nil
}

// sendViaWebhook sends message via incoming webhook
func (p *Provider) sendViaWebhook(ctx context.Context, webhookURL string, notif notification.Notification, blocks []slack.Block) error {
	// Webhook implementation would use net/http to POST to the URL
	// For now, we just return an error indicating it's not implemented
	return fmt.Errorf("%w: webhook not yet implemented", ErrSlackAPIError)
}

// buildApprovalBlocks creates Block Kit blocks for approval requests
func buildApprovalBlocks(notif notification.Notification) []slack.Block {
	var blocks []slack.Block

	// Header
	headerText := slack.NewTextBlockObject("plain_text", notif.Title, true, false)
	headerBlock := slack.NewHeaderBlock(headerText)
	blocks = append(blocks, headerBlock)

	// Context with request details
	contextFields := []*slack.TextBlockObject{
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Request ID:* %s", notif.Context.RequestID), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Secret Path:* `%s`", notif.Context.SecretPath), false, false),
	}
	if notif.Context.RequestedBy != "" {
		contextFields = append(contextFields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Requested By:* %s", notif.Context.RequestedBy), false, false))
	}
	contextBlock := slack.NewContextBlock("", contextFields...)
	blocks = append(blocks, contextBlock)

	// Divider
	blocks = append(blocks, slack.NewDividerBlock())

	// Message
	if notif.Message != "" {
		messageBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", notif.Message, false, false),
			nil,
			nil,
		)
		blocks = append(blocks, messageBlock)
	}

	// Action buttons
	var actionElements []slack.BlockElement
	
	// Approve button
	approveButton := slack.NewButtonBlockElement(
		"approve_action",
		notif.Context.RequestID,
		slack.NewTextBlockObject("plain_text", "Approve", false, false),
	)
	approveButton.Style = slack.StylePrimary
	actionElements = append(actionElements, approveButton)

	// Deny button
	denyButton := slack.NewButtonBlockElement(
		"deny_action",
		notif.Context.RequestID,
		slack.NewTextBlockObject("plain_text", "Deny", false, false),
	)
	denyButton.Style = slack.StyleDanger
	actionElements = append(actionElements, denyButton)

	actionsBlock := slack.NewActionBlock("approval_actions", actionElements...)
	blocks = append(blocks, actionsBlock)

	return blocks
}

// buildAlertBlocks creates Block Kit blocks for system alerts
func buildAlertBlocks(notif notification.Notification) []slack.Block {
	var blocks []slack.Block

	// Header with alert emoji
	priorityEmoji := getPriorityEmoji(notif.Priority)
	headerText := slack.NewTextBlockObject("plain_text", fmt.Sprintf("%s %s", priorityEmoji, notif.Title), true, false)
	headerBlock := slack.NewHeaderBlock(headerText)
	blocks = append(blocks, headerBlock)

	// Alert message
	alertBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Alert:* %s", notif.Message), false, false),
		nil,
		nil,
	)
	blocks = append(blocks, alertBlock)

	return blocks
}

// buildDefaultBlocks creates default Block Kit blocks
func buildDefaultBlocks(notif notification.Notification) []slack.Block {
	var blocks []slack.Block

	// Header
	headerText := slack.NewTextBlockObject("plain_text", notif.Title, true, false)
	headerBlock := slack.NewHeaderBlock(headerText)
	blocks = append(blocks, headerBlock)

	// Message
	if notif.Message != "" {
		messageBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", notif.Message, false, false),
			nil,
			nil,
		)
		blocks = append(blocks, messageBlock)
	}

	return blocks
}

// getPriorityEmoji returns an emoji based on priority level
func getPriorityEmoji(priority int) string {
	switch priority {
	case notification.PriorityCritical:
		return ":red_circle:"
	case notification.PriorityHigh:
		return ":warning:"
	case notification.PriorityMedium:
		return ":information_source:"
	default:
		return ":white_check_mark:"
	}
}

// Close cleans up any resources used by the provider
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		p.client = nil
	}

	if p.rateLimiter != nil {
		// Rate limiter goroutine will stop when tokens channel is garbage collected
		p.rateLimiter = nil
	}

	p.configured = false
	return nil
}
