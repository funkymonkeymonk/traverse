# Traverse - Cross the Security Chasm with Confidence

> *"You wouldn't leap across a canyon without a rope. Don't let your agents access secrets without a safety line."*

## Overview

**Traverse** is your secure rope across the security chasm. It's a self-hosted secrets proxy that adds Multi-Factor Authentication (MFA) and approval workflows to arbitrary secret backends. 

Like a rope swing that carries you safely from one cliff to another, Traverse bridges the gap between your AI agents (on one side) and your sensitive secrets (on the other), with you holding the rope — approving each crossing.

## The Chasm Problem

```
    AI Agents                    You                    Secrets
         │                        │                        │
         │    ╔═══════════════════╗                       │
         │    ║                   ║                       │
         │    ║   THE SECURITY    ║                       │
         │────║      CHASM        ║──────────────────────▶│
         │    ║                   ║    ❌ Direct access    │
         │    ║   No visibility   ║    is too dangerous   │
         │    ╚═══════════════════╝                       │
         │                        │                        │
         ▼                        ▼                        ▼
   Untrusted side           The gap                   Trusted side
```

**The Problem:** Your AI agents need access to production secrets (API keys, database passwords, certificates), but giving them direct access is like letting them leap across a canyon — one misstep and your secrets are exposed.

## The Traverse Solution

```
    AI Agents                    You                    Secrets
         │                        │                        │
         │    ╔═══════════════════════════════════════════╗
         │    ║                                           ║
         │    ║   TRAVERSE: Your Safety Line             ║
         │    ║   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   ║
         │    ║                                           ║
         │────║───▶ Request ──▶ Approve? ──▶ Grant ─────▶│
         │    ║         │          │           │         │
         │    ║         │          ▼           │         │
         │    ║         │    📱 Notification   │         │
         │    ║         │          │           │         │
         │    ║         │          ▼           │         │
         │    ║         │   ✅ You approve    │         │
         │    ║         │          │           │         │
         │◀───║─────────│──────────│───────────│─────────│
         │    ║   Token │          │           │         │
         │    ║   (time-limited)   │           │         │
         │    ║                                           ║
         │    ╚═══════════════════════════════════════════╝
         │                        │                        │
         ▼                        ▼                        ▼
   Untrusted side              Safe passage             Trusted side
```

**The Solution:** Traverse acts as your safety line. When an agent needs a secret:
1. 🎯 They grab the rope (send a request)
2. 📱 You feel the tug (get a notification)
3. ✅ You approve the crossing (click approve)
4. 🎫 They get a time-limited pass (short-lived token)
5. 🏃 They cross safely (access the secret)
6. ⏰ The pass expires (automatic cleanup)

## Core Value Proposition

- **🌉 Bridge Any Gap**: Works with any secret backend (1Password, Vault, AWS, Azure, GCP) via plugin architecture
- **📱 You're In Control**: Request → Notify → Approve → Grant pattern keeps you in the loop
- **🔐 MFA Everywhere**: Duo, TOTP, Pushover, Slack, Telegram, custom webhooks
- **📊 Full Visibility**: Complete audit trail of who accessed what and when
- **🏠 Self-Hosted**: You own the rope — full control over your infrastructure
- **⏱️ Time-Bound**: Automatic expiration means no lingering access

## Architecture

```
                          ┌─────────────────────────────────────┐
                          │           Traverse Proxy            │
                          │  ┌──────────┐   ┌──────────────┐   │
    ┌──────────┐          │  │ Request  │   │   Approval   │   │          ┌───────────────┐
    │  Client  │──────────│──│  Manager │───│   Engine     │───│──────────▶│   1Password   │
    │(AI Agent)│ Request  │  └──────────┘   └──────────────┘   │  Fetch   │    Vault      │
    └──────────┘          │                                    │  Secret  │   AWS, etc.   │
          │               │  ┌──────────────┐ ┌─────────────┐  │          └───────────────┘
          │               │  │   Token      │ │   Audit     │  │
          │               │  │   Service    │ │   Logger    │  │
          │               │  └──────────────┘ └─────────────┘  │
          │               └─────────────────────────────────────┘
          │                            │
          │                            │
          │                            ▼
          │                   ┌──────────────────┐
          │                   │  Notification    │
          │                   │  (Duo, Slack,    │
          │                   │   Pushover...)   │
          │                   └────────┬─────────┘
          │                            │
          │                            ▼
          │                   ┌──────────────────┐
          │                   │   📱 You         │
          │                   │   (Approver)     │
          │                   └────────┬─────────┘
          │                            │
          │ Token (if approved)        │ Approval
          │◀───────────────────────────│
          ▼
    ┌──────────┐
    │  Secret  │
    └──────────┘
```

## Components

### 1. 🎯 Request Manager
The anchor point on the client side. Handles incoming requests, validates permissions, and creates pending request records.

- Queue and track pending requests
- Enforce approval policies (single approver, multi-approver, quorum)
- Handle request expiration and cleanup
- Request deduplication

### 2. 📢 Notification Service (Pluggable)
The rope that tugs your pocket. Sends alerts through your preferred channels.

**Default Providers:**
- **Duo Push** - The classic phone tap
- **Pushover** - Instant push notifications
- **Slack DM** - For teams living in Slack
- **Telegram Bot** - For the privacy-conscious
- **Webhook** - Build your own
- **Email** - Always works

**Example Notification:**
```json
{
  "request_id": "trv_abc123xyz",
  "client_id": "agent-001",
  "secret_path": "prod/api-keys/stripe",
  "requested_at": "2026-05-08T10:00:00Z",
  "expires_at": "2026-05-08T10:05:00Z",
  "reason": "Deploying payment feature",
  "approve_url": "https://traverse.internal/approve/trv_abc123xyz",
  "deny_url": "https://traverse.internal/deny/trv_abc123xyz"
}
```

### 3. 🔌 Provider Interface (Plugin System)
The other anchor point — connects to any secret backend.

**Interface Definition:**
```go
type SecretProvider interface {
    Name() string
    Get(ctx context.Context, path string) (Secret, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Validate(config map[string]interface{}) error
    Health(ctx context.Context) error
}
```

**Built-in Providers:**
1. **1Password Connect** - Your trusted vault
2. **HashiCorp Vault** - Enterprise standard
3. **AWS Secrets Manager** - Cloud native
4. **Azure Key Vault** - Microsoft shop
5. **GCP Secret Manager** - Google cloud
6. **Local File** - Encrypted with age/SOPS
7. **Environment Variables** - Simple and local

### 4. 🎫 Token Service
Your time-limited crossing pass.

- Issue short-lived JWTs for approved requests
- Token contains:
  - Request ID
  - Allowed paths
  - Expiration time
  - Client identity
- Token revocation capability (emergency stop!)

### 5. 📊 Audit Logger
The breadcrumb trail showing who crossed when.

- Structured logging (JSON)
- Events tracked:
  - `REQUEST_CREATED` - Someone grabbed the rope
  - `NOTIFICATION_SENT` - You got the tug
  - `REQUEST_APPROVED` - You let them cross
  - `REQUEST_DENIED` - You stopped them
  - `REQUEST_EXPIRED` - Time ran out
  - `SECRET_ACCESSED` - They made it across
  - `TOKEN_ISSUED` - Pass created
  - `TOKEN_REVOKED` - Emergency brake pulled

## API Specification

### Grab the Rope (Request a Secret)

**POST** `/v1/secrets/request`

**Headers:**
```
Authorization: Bearer <client-api-key>
Content-Type: application/json
```

**Request Body:**
```json
{
  "secret_path": "prod/api-keys/stripe",
  "client_id": "agent-001",
  "reason": "Deploying payment feature PR #1234",
  "requested_duration": "1h",
  "notification_preferences": ["duo", "slack"],
  "metadata": {
    "git_commit": "abc123",
    "pipeline_id": "456"
  }
}
```

**Response (202 Accepted - Rope is ready):**
```json
{
  "request_id": "trv_abc123xyz",
  "status": "pending_approval",
  "message": "Request submitted. Approval notification sent.",
  "poll_url": "/v1/requests/trv_abc123xyz/status",
  "expires_at": "2026-05-08T10:05:00Z"
}
```

### Check the Rope Tension (Get Request Status)

**GET** `/v1/requests/{request_id}/status`

**Response 200 OK (Waiting for you):**
```json
{
  "request_id": "trv_abc123xyz",
  "status": "pending",
  "created_at": "2026-05-08T10:00:00Z",
  "expires_at": "2026-05-08T10:05:00Z",
  "approvers_notified": ["admin@company.com"],
  "approval_count": 0,
  "required_approvals": 1
}
```

**Response 200 OK (Approved - Safe to cross!):**
```json
{
  "request_id": "trv_abc123xyz",
  "status": "approved",
  "approved_at": "2026-05-08T10:01:30Z",
  "approved_by": ["admin@company.com"],
  "token": "eyJhbGc...",
  "token_expires_at": "2026-05-08T11:01:30Z",
  "secret_url": "/v1/secrets/prod/api-keys/stripe?token=eyJhbGc..."
}
```

### Cross the Chasm (Retrieve Secret)

**GET** `/v1/secrets/{path}?token={jwt}`

**Response:**
```json
{
  "path": "prod/api-keys/stripe",
  "values": {
    "api_key": "sk_live_...",
    "webhook_secret": "whsec_..."
  },
  "metadata": {
    "version": "3",
    "last_rotated": "2026-04-01T00:00:00Z"
  },
  "access_granted_at": "2026-05-08T10:01:30Z",
  "access_expires_at": "2026-05-08T11:01:30Z"
}
```

## Configuration

### Server Configuration (`traverse.yaml`)

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls:
    cert_file: "/etc/traverse/server.crt"
    key_file: "/etc/traverse/server.key"
  
  # Rate limiting
  rate_limit:
    requests_per_minute: 60
    burst: 10

# Authentication for clients
auth:
  type: "api_key"  # or "jwt", "mtls"
  api_keys:
    - key: "${CLIENT_API_KEY}"
      client_id: "agent-001"
      allowed_paths: ["dev/*", "staging/*"]
      rate_limit: 100

# Secret provider configuration
providers:
  default: "1password"
  
  1password:
    type: "1password-connect"
    host: "http://localhost:8080"
    token: "${OP_CONNECT_TOKEN}"
    
  vault:
    type: "hashicorp-vault"
    address: "https://vault.internal:8200"
    auth:
      type: "approle"
      role_id: "${VAULT_ROLE_ID}"
      secret_id: "${VAULT_SECRET_ID}"

# Notification configuration
notifications:
  default: ["duo", "slack"]
  
  duo:
    integration_key: "${DUO_INTEGRATION_KEY}"
    secret_key: "${DUO_SECRET_KEY}"
    api_hostname: "api-xxxxxxxx.duosecurity.com"
    
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
    approver_channel: "#security-approvals"
    
  pushover:
    app_token: "${PUSHOVER_APP_TOKEN}"
    user_key: "${PUSHOVER_USER_KEY}"

# Approval policies
policies:
  default:
    required_approvals: 1
    max_duration: "24h"
    request_timeout: "5m"
    allow_self_approval: false
    
  # Path-specific policies
  overrides:
    - path_pattern: "prod/*"
      required_approvals: 2
      max_duration: "1h"
      request_timeout: "10m"
      approver_groups: ["ops", "security"]
      
    - path_pattern: "*/database/password"
      required_approvals: 2
      max_duration: "30m"

# Audit logging
audit:
  type: "file"
  file:
    path: "/var/log/traverse/audit.log"
    max_size: 100  # MB
    max_backups: 10
    max_age: 30    # days
  
  # Optional: forward to SIEM
  webhook:
    url: "https://splunk.internal:8088/services/collector/event"
    token: "${SPLUNK_TOKEN}"

# Database for request state (SQLite default, PostgreSQL for HA)
storage:
  type: "sqlite"
  sqlite:
    path: "/var/lib/traverse/traverse.db"
```

## The Journey (Workflow)

### 1. 🎯 Request Phase
*The agent reaches for the rope*
1. Client sends secret request with path and reason
2. Traverse validates client credentials and permissions
3. Traverse checks if client already has valid token for this path
4. Traverse creates pending request record
5. Response returns 202 with request ID

### 2. 📱 Notification Phase
*The rope tugs your pocket*
1. Notification service sends alerts via configured channels
2. Each notification includes:
   - Request details (who, what, why)
   - One-click approve/deny URLs
   - Expiration countdown
3. Multiple notifications can be sent (Duo + Slack)

### 3. ✅ Approval Phase
*You decide if it's safe to cross*
1. Approver receives notification
2. Approver reviews request context
3. Approver clicks approve (or uses CLI/API)
4. Traverse validates approver is authorized
5. Traverse checks approval policy (count, groups)
6. On sufficient approvals, generate JWT token
7. Notify client (if polling) or update status endpoint

### 4. 🏃 Access Phase
*The agent crosses safely*
1. Client retrieves token (via poll or webhook)
2. Client uses token to fetch secret
3. Traverse validates token (signature, expiration, path match)
4. Traverse fetches secret from underlying provider
5. Secret is returned to client
6. Audit log records access

### 5. ⏰ Expiration Phase
*The rope retracts automatically*
1. Token expires after configured duration
2. Subsequent requests with expired token return 401
3. Client must request new approval
4. Audit log records expiration

## Security Considerations

### Threat Model

**Protected Against:**
- 🚫 Unauthorized secret access (requires your approval)
- 🚫 Stolen client credentials (needs approval + MFA)
- 🚫 Insider threats (audit trail, dual approval)
- 🚫 Replay attacks (short-lived tokens)
- 🚫 Long-lived credentials (auto-expiration)

**Requires Additional Protection:**
- ⚠️ Traverse server compromise (use HSM for signing)
- ⚠️ Network interception (use TLS/mTLS)
- ⚠️ Notification spoofing (validate callback signatures)

### Best Practices

1. **🏗️ Run Traverse on isolated infrastructure** - Don't share with untrusted workloads
2. **🔐 Use mTLS between Traverse and providers** - Encrypt the whole bridge
3. **📝 Enable audit logging to immutable storage** - Keep the trail forever
4. **🔄 Regular token rotation for client credentials** - Change the locks
5. **📊 Monitor for unusual request patterns** - Watch for rope tugs at 3 AM
6. **⏱️ Use short request timeouts (5-10 minutes)** - Don't leave the rope hanging
7. **👥 Require multiple approvers for production secrets** - Two-person rule

## Implementation Phases

### Phase 1: MVP (Weeks 1-2)
*Build the first rope bridge*
- Basic HTTP API
- 1Password Connect provider only
- Single Duo notification
- SQLite storage
- File audit logging
- Single approver support

### Phase 2: Enhanced Security (Weeks 3-4)
*Multiple rope types, better anchors*
- Multiple notification providers
- Policy engine with path-based rules
- PostgreSQL support for HA
- Webhook audit forwarding
- Token revocation API
- CLI tool for approvers

### Phase 3: Enterprise Features (Weeks 5-6)
*Heavy-duty bridge for production*
- Additional providers (Vault, AWS, Azure, GCP)
- gRPC support
- Web UI for approvers
- Metrics (Prometheus)
- Request batching
- Emergency break-glass access

### Phase 4: Ecosystem (Weeks 7-8)
*Ropes everywhere*
- Kubernetes operator
- Terraform provider
- GitHub Actions integration
- SDKs (Go, Python, Node)
- Plugin marketplace

## Success Metrics

- ⚡ Request approval latency < 30 seconds (phone approval)
- ⚡ Token issuance latency < 100ms
- 🟢 API availability > 99.9%
- 🔌 Support for 10+ secret providers
- 📋 Audit log completeness 100%

## Future Enhancements

- 🤖 Machine learning for anomaly detection (detect unusual crossing patterns)
- 🔄 Automatic secret rotation coordination
- ☁️ Just-in-time access for cloud IAM
- 📱 Mobile app for approvers
- 🚨 Integration with PagerDuty/Opsgenie
- 📜 Policy as code (Rego/OPA)

## Why "Traverse"?

The name embodies our philosophy: **security is a journey, not a destination.** 

Just as traversing a canyon requires:
- 🎯 A clear starting point (the request)
- 📢 Communication with your team (notifications)
- ✅ Active decision-making (approval)
- ⏱️ Time limits (expiration)
- 📝 Accountability (audit logs)

Traverse provides the safety equipment you need to make that journey confidently.

---

## Appendix A: Provider Implementation Guide

### Creating a Custom Provider

```go
package main

import (
    "context"
    "github.com/funkymonkeymonk/traverse/pkg/provider"
)

type MyProvider struct {
    config map[string]interface{}
}

func (p *MyProvider) Name() string {
    return "my-provider"
}

func (p *MyProvider) Configure(config map[string]interface{}) error {
    p.config = config
    // Validate required fields
    return nil
}

func (p *MyProvider) Get(ctx context.Context, path string) (provider.Secret, error) {
    // Implement secret retrieval
    return provider.Secret{
        Path: path,
        Value: map[string]string{
            "key": "value",
        },
    }, nil
}

func init() {
    provider.Register("my-provider", func() provider.Provider {
        return &MyProvider{}
    })
}
```

## Appendix B: CLI Reference

```bash
# Server
traverse server --config traverse.yaml

# Client
traverse request --path prod/api-keys/stripe --reason "Hotfix deploy"
traverse get --path prod/api-keys/stripe --token <jwt>

# Admin
traverse requests list --status pending
traverse request approve trv_abc123 --reason "Approved for deploy"
traverse request deny trv_abc123 --reason "Not authorized"

# Setup
traverse provider test --name 1password
traverse notification test --provider duo
```

## Appendix C: Docker Compose Example

```yaml
version: '3.8'
services:
  traverse:
    image: funkymonkeymonk/traverse:latest
    ports:
      - "8080:8080"
    volumes:
      - ./traverse.yaml:/etc/traverse/config.yaml:ro
      - ./certs:/etc/traverse/certs:ro
      - traverse-data:/var/lib/traverse
    environment:
      - OP_CONNECT_TOKEN=${OP_CONNECT_TOKEN}
      - DUO_INTEGRATION_KEY=${DUO_INTEGRATION_KEY}
      - DUO_SECRET_KEY=${DUO_SECRET_KEY}
    depends_on:
      - 1password-connect
      
  1password-connect:
    image: 1password/connect-api:latest
    ports:
      - "8081:8080"
    volumes:
      - ./1password-credentials.json:/home/opuser/.op/1password-credentials.json:ro
      - op-data:/home/opuser/.op/data

volumes:
  traverse-data:
  op-data:
```
