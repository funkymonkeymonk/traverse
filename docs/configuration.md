# Configuration Reference

Complete reference for Traverse configuration options.

## Configuration File Structure

```yaml
server:          # Server settings
auth:            # Authentication configuration
providers:       # Secret provider configuration
notifications:   # Notification provider settings
policies:        # Approval policies
storage:         # Database/storage configuration
audit:           # Audit logging configuration
metrics:         # Metrics and monitoring
health:          # Health check configuration
break_glass:     # Emergency access configuration
```

## Server Configuration

```yaml
server:
  host: "0.0.0.0"        # Bind address (default: "0.0.0.0")
  port: 8080             # HTTP port (default: 8080)
  
  # TLS Configuration (optional but recommended)
  tls:
    cert_file: "/etc/traverse/certs/server.crt"
    key_file: "/etc/traverse/certs/server.key"
    client_ca_file: "/etc/traverse/certs/ca.crt"  # For mTLS
    client_auth: "require_and_verify"              # mTLS mode
  
  # Rate Limiting
  rate_limit:
    requests_per_minute: 60   # Requests per minute per client
    burst: 10                 # Burst allowance
  
  # CORS Settings
  cors:
    allowed_origins: ["https://app.company.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Authorization", "Content-Type"]
    max_age: 3600             # Preflight cache duration (seconds)
```

### TLS Client Auth Modes

| Mode | Description |
|------|-------------|
| `none` | No client certificate required |
| `request` | Request but don't require client certificate |
| `require` | Require client certificate |
| `require_and_verify` | Require and verify client certificate (mTLS) |

## Authentication Configuration

### API Key Authentication

```yaml
auth:
  type: "api_key"
  api_keys:
    - key: "api_key_1"                    # The API key (use env var in production)
      client_id: "ci-runner-1"            # Human-readable identifier
      allowed_paths: ["prod/*", "shared/*"]  # Allowed secret paths
      rate_limit: 30                      # Custom rate limit (0 = unlimited)
      
    - key: "${API_KEY_2}"                 # Use environment variable
      client_id: "deployment-bot"
      allowed_paths: ["prod/api-keys/*"]
      rate_limit: 10
```

### mTLS Authentication

```yaml
auth:
  type: "mtls"
  
  # Client certificate to client mapping
  client_certificates:
    - subject_cn: "ci-runner-1.company.com"
      client_id: "ci-runner-1"
      allowed_paths: ["prod/*"]
      
    - subject_cn: "deployment-bot.company.com"
      client_id: "deployment-bot"
      allowed_paths: ["prod/api-keys/*", "prod/database/*"]
```

## Secret Provider Configuration

### 1Password Connect

```yaml
providers:
  default: "1password"
  
  1password:
    type: "1password-connect"
    host: "https://op-connect.internal:8080"
    token: "${OP_CONNECT_TOKEN}"          # 1Password Connect token
    
    # TLS Configuration
    tls:
      ca_cert: "/etc/traverse/certs/op-ca.crt"
      insecure_skip_verify: false         # Never true in production
    
    # Request timeouts
    timeout: "10s"
    
    # Retry configuration
    retry:
      max_retries: 3
      backoff: "exponential"              # "linear" or "exponential"
      initial_delay: "100ms"
      max_delay: "5s"
```

### HashiCorp Vault

```yaml
providers:
  vault:
    type: "hashicorp-vault"
    address: "https://vault.internal:8200"
    
    # Authentication
    auth:
      type: "approle"                     # "token", "approle", "kubernetes"
      
      # AppRole auth
      role_id: "${VAULT_ROLE_ID}"
      secret_id: "${VAULT_SECRET_ID}"
      
      # Or token auth
      # token: "${VAULT_TOKEN}"
      
      # Or Kubernetes auth
      # kubernetes:
      #   role: "traverse"
      #   service_account_path: "/var/run/secrets/kubernetes.io/serviceaccount/token"
    
    # TLS
    tls:
      ca_cert: "/etc/traverse/certs/vault-ca.crt"
    
    # Engine version
    kv_version: "v2"                      # "v1" or "v2"
```

### AWS Secrets Manager

```yaml
providers:
  aws:
    type: "aws-secrets-manager"
    region: "us-east-1"
    
    # Authentication (uses default credential chain if not specified)
    # auth:
    #   access_key_id: "${AWS_ACCESS_KEY_ID}"
    #   secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
    #   session_token: "${AWS_SESSION_TOKEN}"
    
    # Or use IAM role
    # role_arn: "arn:aws:iam::123456789:role/traverse-role"
```

### Local File Provider

```yaml
providers:
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"
    
    # Encryption (optional but recommended)
    encryption:
      type: "age"                         # "age" or "pgp"
      recipient: "age1ql3z7hjy54pw3..."   # Age public key
      # Or for PGP:
      # public_key: "/etc/traverse/keys/public.asc"
      # private_key: "/etc/traverse/keys/private.asc"
```

### Multiple Providers

```yaml
providers:
  default: "1password"
  
  1password:
    type: "1password-connect"
    # ... config
  
  vault:
    type: "hashicorp-vault"
    # ... config
  
  local:
    type: "local"
    # ... config
```

## Notification Provider Configuration

### Duo Push

```yaml
notifications:
  default: ["duo"]
  
  duo:
    integration_key: "${DUO_INTEGRATION_KEY}"
    secret_key: "${DUO_SECRET_KEY}"
    api_hostname: "api-xxxxxxxx.duosecurity.com"
    fail_open: false                      # Allow request if Duo is down
    timeout: "30s"
```

### Slack

```yaml
notifications:
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"       # xoxb-... format
    approver_channel: "#security-approvals"
    dm_users: true                        # Also send DMs to approvers
    
    # Message customization
    templates:
      request:
        title: "🔐 Secret Access Request"
        color: "#36a64f"
      urgent:
        title: "🚨 URGENT: Secret Access"
        color: "#ff0000"
```

### PagerDuty

```yaml
notifications:
  pagerduty:
    integration_key: "${PAGERDUTY_KEY}"
    severity: "critical"
    only_for_paths: ["prod/database/*", "prod/admin/*"]
```

### Telegram

```yaml
notifications:
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    chat_ids: [123456789, -987654321]     # User and group chat IDs
```

### Email (SMTP)

```yaml
notifications:
  email:
    smtp_host: "smtp.company.com"
    smtp_port: 587
    smtp_username: "${SMTP_USER}"
    smtp_password: "${SMTP_PASSWORD}"
    from_address: "traverse@company.com"
    use_tls: true
    
    # Default recipients (can be overridden per-policy)
    default_recipients: ["security@company.com"]
```

### Webhook

```yaml
notifications:
  webhook:
    url: "https://api.company.com/traverse-webhook"
    secret: "${WEBHOOK_SECRET}"           # For HMAC signature
    headers:
      X-Custom-Header: "value"
    timeout: "10s"
    retry:
      max_retries: 3
```

### Multiple Notification Channels

```yaml
notifications:
  default: ["slack", "duo"]
  
  slack:
    # ... config
  
  duo:
    # ... config
  
  pagerduty:
    # ... config (used for critical paths only)
```

## Approval Policies

### Default Policy

```yaml
policies:
  default:
    required_approvals: 1                 # Number of approvals needed
    max_duration: "1h"                    # Max access duration
    request_timeout: "5m"                 # How long to wait for approval
    allow_self_approval: false            # Can requester approve own request?
    require_justification: true           # Require reason for access
    min_justification_length: 20          # Minimum justification length
    
    # Who can approve
    approver_groups: ["ops-team", "security-team"]
    
    # Notification channels for this policy
    notification_channels: ["slack", "duo"]
```

### Path-Specific Overrides

```yaml
policies:
  default:
    # ... default config
  
  overrides:
    # Production secrets require dual approval
    - path_pattern: "prod/*"
      required_approvals: 2
      max_duration: "30m"
      approver_groups: ["senior-ops", "security-team"]
      allow_self_approval: false
      
    # Database secrets have shorter timeout
    - path_pattern: "prod/database/*"
      max_duration: "15m"
      request_timeout: "10m"
      approver_groups: ["dba-oncall"]
      notification_channels: ["pagerduty", "slack"]
      
    # Admin access requires 3 approvers
    - path_pattern: "*/admin"
      required_approvals: 3
      max_duration: "10m"
      approver_groups: ["senior-staff"]
      
    # Shared secrets are more lenient
    - path_pattern: "shared/*"
      required_approvals: 1
      max_duration: "8h"
      allow_self_approval: true
```

### Path Pattern Syntax

| Pattern | Matches |
|---------|---------|
| `*` | Any single path segment |
| `**` | Any number of segments |
| `prod/*` | All direct children of `prod` |
| `prod/**` | All descendants of `prod` |
| `*/admin` | Any `admin` at any depth |

## Storage Configuration

### SQLite (Development)

```yaml
storage:
  type: "sqlite"
  sqlite:
    path: "/var/lib/traverse/traverse.db"
    # Connection pool settings
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: "1h"
```

### PostgreSQL (Production)

```yaml
storage:
  type: "postgresql"
  postgresql:
    host: "postgres.internal"
    port: 5432
    database: "traverse"
    user: "traverse"
    password: "${DB_PASSWORD}"
    
    # Connection pool
    max_connections: 20
    max_idle: 5
    
    # SSL/TLS
    ssl_mode: "verify-full"              # "disable", "require", "verify-ca", "verify-full"
    ssl_cert: "/etc/traverse/certs/postgres-client.crt"
    ssl_key: "/etc/traverse/certs/postgres-client.key"
    ssl_root_cert: "/etc/traverse/certs/postgres-ca.crt"
    
    # Cleanup
    cleanup:
      enabled: true
      retention_days: 90
      schedule: "0 3 * * *"              # Daily at 3 AM
```

## Audit Logging Configuration

### File Audit Log

```yaml
audit:
  type: "file"
  file:
    path: "/var/log/traverse/audit.log"
    max_size: 100                         # MB
    max_backups: 30                       # Number of old logs to keep
    max_age: 90                           # Days
    compress: true                        # Compress old logs
    format: "json"                        # "json" or "text"
```

### Syslog

```yaml
audit:
  type: "syslog"
  syslog:
    network: "tcp"                        # "tcp", "udp", or "unix"
    address: "syslog.internal:514"
    tag: "traverse"
    severity: "info"                      # syslog severity level
```

### Webhook

```yaml
audit:
  type: "webhook"
  webhook:
    url: "https://siem.company.com/api/events"
    headers:
      Authorization: "Bearer ${SIEM_TOKEN}"
      X-Source: "traverse"
    filter_events:                        # Only send these events
      - "REQUEST_CREATED"
      - "REQUEST_APPROVED"
      - "SECRET_ACCESSED"
      - "REQUEST_DENIED"
    timeout: "5s"
    retry:
      max_retries: 3
      backoff: "exponential"
```

### Multiple Destinations

```yaml
audit:
  type: "multi"
  destinations:
    - type: "file"
      file:
        path: "/var/log/traverse/audit.log"
    - type: "syslog"
      syslog:
        address: "syslog.internal:514"
    - type: "webhook"
      webhook:
        url: "https://siem.company.com/api/events"
```

### Event Types

| Event | Description |
|-------|-------------|
| `SERVER_START` | Server started |
| `SERVER_STOP` | Server stopped |
| `REQUEST_CREATED` | Access request created |
| `REQUEST_APPROVED` | Request approved |
| `REQUEST_DENIED` | Request denied |
| `REQUEST_EXPIRED` | Request expired |
| `SECRET_ACCESSED` | Secret accessed |
| `SECRET_NOT_FOUND` | Secret not found (access denied) |
| `AUTH_FAILURE` | Authentication failure |
| `POLICY_VIOLATION` | Policy violation |

## Metrics Configuration

```yaml
metrics:
  enabled: true
  type: "prometheus"
  prometheus:
    port: 9090
    path: "/metrics"
    
    # Additional labels for all metrics
    labels:
      environment: "production"
      cluster: "primary"
      datacenter: "us-east-1"
```

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `traverse_requests_total` | Counter | Total access requests |
| `traverse_requests_duration_seconds` | Histogram | Request duration |
| `traverse_secrets_accessed_total` | Counter | Secrets accessed |
| `traverse_approvals_total` | Counter | Approvals/denials |
| `traverse_active_requests` | Gauge | Currently pending requests |
| `traverse_provider_health` | Gauge | Provider health status |

## Health Check Configuration

```yaml
health:
  enabled: true
  port: 8081                              # Separate from main port
  path: "/health"
  detailed: true                          # Include provider health
  
  # Additional checks
  checks:
    database: true
    providers: true
```

### Health Response

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "version": "1.0.0",
  "checks": {
    "database": {"status": "healthy", "latency_ms": 5},
    "providers": {
      "1password": {"status": "healthy"},
      "vault": {"status": "healthy"}
    }
  }
}
```

## Emergency Break-Glass Configuration

```yaml
break_glass:
  enabled: true
  
  # Special admin token (store in HSM or hardware token)
  admin_token: "${BREAK_GLASS_TOKEN}"
  
  # Require hardware token (HSM, YubiKey)
  require_hardware_token: true
  
  # Separate audit severity
  audit_severity: "critical"
  
  # Immediate notifications
  notify: 
    - "security@company.com"
    - "#security-alerts"
```

## Environment Variables

All configuration values can be overridden using environment variables:

```bash
# Server settings
export TRAVERSE_SERVER_HOST="0.0.0.0"
export TRAVERSE_SERVER_PORT="8080"

# Provider settings
export TRAVERSE_PROVIDERS_1PASSWORD_TOKEN="op_token_here"
export TRAVERSE_PROVIDERS_VAULT_TOKEN="vault_token_here"

# Notification settings
export TRAVERSE_NOTIFICATIONS_SLACK_BOT_TOKEN="xoxb-..."
export TRAVERSE_NOTIFICATIONS_DUO_INTEGRATION_KEY="..."

# Database settings
export TRAVERSE_STORAGE_POSTGRESQL_PASSWORD="db_password"
```

Variable naming convention: `TRAVERSE_<SECTION>_<KEY>` with underscores replacing nested keys.

## Complete Example

See [examples/config.production.yaml](../examples/config.production.yaml) for a complete production-ready configuration.
