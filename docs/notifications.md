# Notification Setup

Configure notification providers for approval workflows.

## Supported Providers

- [Duo Security](#duo-security)
- [Slack](#slack)
- [PagerDuty](#pagerduty)
- [Telegram](#telegram)
- [Email (SMTP)](#email-smtp)
- [Webhook](#webhook)

---

## Duo Security

Duo Push provides interactive approval with approve/deny buttons.

### Prerequisites

- Duo account with Admin API access
- Integration key and secret key

### Setup Steps

#### 1. Create Duo Application

1. Log in to Duo Admin Panel
2. Navigate to **Applications** → **Protect an Application**
3. Search for "Auth API"
4. Click **Protect**
5. Note the **Integration Key**, **Secret Key**, and **API Hostname**

#### 2. Configure Users

Ensure approvers have Duo accounts:

1. Go to **Users** in Duo Admin Panel
2. Add or verify approvers exist
3. Note their **Username** (usually email)

#### 3. Configure Traverse

```yaml
notifications:
  default: ["duo"]
  
  duo:
    integration_key: "${DUO_INTEGRATION_KEY}"
    secret_key: "${DUO_SECRET_KEY}"
    api_hostname: "api-xxxxxxxx.duosecurity.com"
    fail_open: false              # Deny if Duo is unavailable
    timeout: "30s"
```

#### 4. Map Approvers

```yaml
policies:
  default:
    approvers:
      - identity: "admin@company.com"
        name: "Admin User"
        contact_info: "admin@company.com"    # Duo username
        channels: ["duo"]
        
      - identity: "ops@company.com"
        name: "Ops Team"
        contact_info: "ops@company.com"
        channels: ["duo", "slack"]
```

### How It Works

1. Request is created
2. Duo Push notification sent to approver's device
3. Approver taps "Approve" or "Deny"
4. Traverse polls Duo for response
5. Secret is released or request is denied

### Testing

```bash
# Test Duo integration
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "duo",
    "recipient": "admin@company.com"
  }'
```

### Troubleshooting

**Push Not Received**
- Verify approver's Duo username matches `contact_info`
- Check Duo Mobile app is installed and enrolled
- Ensure device has internet connectivity

**Invalid Integration Key**
```bash
# Verify credentials
curl -s "https://api-xxxxxxxx.duosecurity.com/auth/v2/check" \
  -H "Date: $(date -u '+%a, %d %b %Y %H:%M:%S %Z')" \
  -H "Authorization: Basic $(echo -n 'INTEGRATION_KEY:SECRET_KEY' | base64)"
```

---

## Slack

Slack notifications support interactive approval buttons and direct messages.

### Prerequisites

- Slack workspace with admin access
- Bot token with appropriate scopes

### Setup Steps

#### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App** → **From scratch**
3. Name it "Traverse" and select your workspace
4. Click **Create App**

#### 2. Configure OAuth Scopes

Navigate to **OAuth & Permissions**:

Required Bot Token Scopes:
- `chat:write` - Send messages
- `chat:write.public` - Send messages to public channels
- `im:write` - Open direct messages
- `users:read` - Read user info
- `users:read.email` - Read user email addresses

Optional:
- `channels:read` - List channels
- `groups:read` - List private channels

#### 3. Enable Interactivity

Navigate to **Interactivity & Shortcuts**:

1. Turn **On** Interactivity
2. Set Request URL: `https://traverse.company.com/webhooks/slack`
3. Add actions:
   - `approve_request`
   - `deny_request`

#### 4. Install App to Workspace

1. Go to **OAuth & Permissions**
2. Click **Install to Workspace**
3. Copy the **Bot User OAuth Token** (starts with `xoxb-`)

#### 5. Invite Bot to Channels

```
/invite @Traverse #security-approvals
```

#### 6. Configure Traverse

```yaml
notifications:
  default: ["slack"]
  
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
    approver_channel: "#security-approvals"
    dm_users: true              # Send DMs to approvers
    
    # Message templates
    templates:
      request:
        title: "🔐 Secret Access Request"
        color: "#36a64f"
      urgent:
        title: "🚨 URGENT: Secret Access"
        color: "#ff0000"
```

#### 7. Configure Slack Callback URL

```yaml
server:
  host: "0.0.0.0"
  port: 8080

# Slack webhook endpoint is automatically available at:
# POST /webhooks/slack
```

### Mapping Approvers

```yaml
policies:
  default:
    approvers:
      - identity: "admin@company.com"
        name: "Admin User"
        contact_info: "U0123456789"      # Slack User ID
        channels: ["slack"]
        
      - identity: "ops@company.com"
        name: "Ops Team"
        contact_info: "@ops-team"         # Can use @username
        channels: ["slack"]
```

**Finding Slack User IDs:**

```bash
# Method 1: From Slack
# Click user's profile → three dots → "Copy member ID"

# Method 2: Via API
curl -s "https://slack.com/api/users.lookupByEmail" \
  -H "Authorization: Bearer xoxb-..." \
  --data-urlencode "email=admin@company.com" | jq '.user.id'
```

### Message Format

Example Slack message:

```
🔐 Secret Access Request

Client: deployment-bot
Path: prod/database/password
Reason: Database migration for v2.0
Duration: 30 minutes
Expires: 5 minutes

[✅ Approve] [❌ Deny] [👁️ View Details]
```

### Testing

```bash
# Test Slack notification
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "slack",
    "recipient": "#security-approvals"
  }'
```

### Troubleshooting

**Bot Can't Post to Channel**
- Invite bot with `/invite @Traverse`
- Check bot has `chat:write` scope
- Verify channel name is correct

**Interactive Buttons Not Working**
- Ensure Request URL is correct and publicly accessible
- Check TLS certificate is valid
- Verify URL is reachable from Slack's servers

**DMs Not Sending**
- Enable `im:write` scope
- User must have interacted with bot first, or use User ID not @username

---

## PagerDuty

PagerDuty integration for high-priority requests requiring immediate attention.

### Prerequisites

- PagerDuty account
- Integration key for a service

### Setup Steps

#### 1. Create Integration

1. Log in to PagerDuty
2. Go to **Services** → Select your service
3. Click **Integrations** tab
4. Click **Add Integration**
5. Select **Events API v2**
6. Copy the **Integration Key**

#### 2. Configure Traverse

```yaml
notifications:
  pagerduty:
    integration_key: "${PAGERDUTY_KEY}"
    severity: "critical"              # "info", "warning", "error", "critical"
    only_for_paths:                   # Only notify for these paths
      - "prod/database/*"
      - "prod/admin/*"
```

#### 3. Path-Specific Configuration

```yaml
policies:
  default:
    notification_channels: ["slack"]
    
  overrides:
    - path_pattern: "prod/database/*"
      notification_channels: ["pagerduty", "slack"]
```

### How It Works

1. Request is created for sensitive path
2. PagerDuty incident is created
3. On-call engineer is notified
4. Engineer approves via PagerDuty app or web UI
5. Webhook callback updates Traverse

### Testing

```bash
# Test PagerDuty integration
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "pagerduty",
    "severity": "critical"
  }'
```

---

## Telegram

Telegram bot notifications with inline keyboards for approval.

### Prerequisites

- Telegram account
- Bot token from @BotFather

### Setup Steps

#### 1. Create Telegram Bot

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot`
3. Name your bot (e.g., "TraverseBot")
4. Choose username (e.g., "@YourTraverseBot")
5. Copy the **HTTP API Token**

#### 2. Get Chat ID

```bash
# Message your bot first, then:
curl -s "https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates" | \
  jq '.result[0].message.chat.id'
```

Or for groups:
- Add bot to group
- Send a message
- Use same API call to get group chat ID (negative number)

#### 3. Configure Traverse

```yaml
notifications:
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    chat_ids: [123456789, -987654321]   # User and group IDs
    
    # Optional: webhook for interactive buttons
    webhook_url: "https://traverse.company.com/webhooks/telegram"
```

#### 4. Set Webhook (Optional)

```bash
# If using webhooks for interactive buttons
curl -s "https://api.telegram.org/bot<TOKEN>/setWebhook" \
  -d "url=https://traverse.company.com/webhooks/telegram"
```

### Mapping Approvers

```yaml
policies:
  default:
    approvers:
      - identity: "admin@company.com"
        name: "Admin User"
        contact_info: "123456789"      # Telegram User ID
        channels: ["telegram"]
```

### Testing

```bash
# Test Telegram notification
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "telegram",
    "recipient": "123456789"
  }'
```

---

## Email (SMTP)

Email notifications with approval links.

### Prerequisites

- SMTP server access
- Valid sending email address

### Setup Steps

#### 1. Configure SMTP

```yaml
notifications:
  email:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    smtp_username: "${SMTP_USER}"
    smtp_password: "${SMTP_PASSWORD}"
    from_address: "traverse@company.com"
    use_tls: true
    
    default_recipients:
      - "security@company.com"
      - "ops-oncall@company.com"
```

#### 2. Gmail Setup

For Gmail, use an App Password:

1. Go to Google Account → Security
2. Enable 2-Step Verification
3. Go to **App passwords**
4. Select **Mail** and your device
5. Copy the 16-character password
6. Use this as `SMTP_PASSWORD`

#### 3. Configure Recipients

```yaml
policies:
  default:
    approvers:
      - identity: "security@company.com"
        name: "Security Team"
        contact_info: "security@company.com"
        channels: ["email"]
```

### Email Template

```
Subject: Secret Access Request - prod/database/password

A secret access request requires your approval.

Request Details:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Client:    deployment-bot
Path:      prod/database/password
Reason:    Database migration for v2.0
Duration:  30 minutes
Expires:   2024-01-15 10:30 UTC
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Approve Request] [Deny Request]

View full details: https://traverse.company.com/requests/abc123
```

### Testing

```bash
# Test email notification
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "email",
    "recipient": "admin@company.com"
  }'
```

---

## Webhook

Custom webhook integration for any notification service.

### Prerequisites

- Webhook endpoint URL
- Shared secret for HMAC verification (optional)

### Setup Steps

#### 1. Configure Webhook

```yaml
notifications:
  webhook:
    url: "https://api.company.com/traverse-webhook"
    secret: "${WEBHOOK_SECRET}"         # For HMAC signature
    headers:
      X-Custom-Header: "value"
      Authorization: "Bearer ${INTERNAL_TOKEN}"
    timeout: "10s"
    retry:
      max_retries: 3
      backoff: "exponential"
```

#### 2. Webhook Payload

Traverse sends POST requests with this JSON payload:

```json
{
  "event": "request_created",
  "timestamp": "2024-01-15T10:00:00Z",
  "request": {
    "id": "req_abc123",
    "client_id": "deployment-bot",
    "secret_path": "prod/database/password",
    "reason": "Database migration",
    "duration": "30m",
    "expires_at": "2024-01-15T10:05:00Z"
  },
  "approvers": [
    {
      "identity": "admin@company.com",
      "name": "Admin User"
    }
  ],
  "actions": {
    "approve": "https://traverse.company.com/v1/requests/req_abc123/approve",
    "deny": "https://traverse.company.com/v1/requests/req_abc123/deny"
  }
}
```

#### 3. HMAC Verification

If `secret` is configured, webhook includes HMAC signature:

```
X-Traverse-Signature: sha256=<hex_signature>
```

Verify in your webhook handler:

```python
import hmac
import hashlib

def verify_webhook(payload, signature, secret):
    expected = hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)
```

#### 4. Response Callback

Your webhook should call back to Traverse when user approves/denies:

```bash
# Approve
curl -X POST https://traverse.company.com/v1/requests/req_abc123/approve \
  -H "Authorization: Bearer WEBHOOK_TOKEN" \
  -d '{
    "approver": "admin@company.com",
    "reason": "Approved via custom UI"
  }'

# Deny
curl -X POST https://traverse.company.com/v1/requests/req_abc123/deny \
  -H "Authorization: Bearer WEBHOOK_TOKEN" \
  -d '{
    "approver": "admin@company.com",
    "reason": "Not authorized"
  }'
```

### Testing

```bash
# Test webhook
curl -X POST http://localhost:8080/v1/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "provider": "webhook",
    "url": "https://webhook.site/your-unique-url"
  }'
```

---

## Multiple Providers

Configure multiple notification channels for redundancy:

```yaml
notifications:
  default: ["slack", "duo"]
  
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
    approver_channel: "#security-approvals"
    dm_users: true
    
  duo:
    integration_key: "${DUO_INTEGRATION_KEY}"
    secret_key: "${DUO_SECRET_KEY}"
    api_hostname: "api-xxxxxxxx.duosecurity.com"
    
  pagerduty:
    integration_key: "${PAGERDUTY_KEY}"
    severity: "critical"
```

**Order matters**: Providers are tried in order until one succeeds.

### Provider Selection by Path

```yaml
policies:
  default:
    notification_channels: ["slack"]
    
  overrides:
    - path_pattern: "prod/database/*"
      notification_channels: ["pagerduty", "slack", "email"]
      
    - path_pattern: "dev/*"
      notification_channels: ["slack"]
```

---

## Fallback Strategy

Configure fallbacks for critical requests:

```yaml
policies:
  default:
    required_approvals: 1
    notification_channels: ["slack"]
    
    # Escalate if not responded in 5 minutes
    escalation:
      enabled: true
      delay: "5m"
      channels: ["pagerduty", "email"]
```

This sends initial notification via Slack, and if not approved within 5 minutes, escalates to PagerDuty and email.
