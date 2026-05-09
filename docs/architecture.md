# Architecture Overview

This document provides a technical overview of Traverse's architecture, design decisions, and system components.

## System Overview

Traverse is a Go-based HTTP proxy service that adds MFA and approval workflows to existing secret management systems. It operates as a middleware layer between clients (applications, CI/CD pipelines) and secret providers (1Password, Vault, etc.).

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Client App    │────▶│    Traverse      │────▶│  1Password /    │
│  (CI/CD, App)   │◀────│  (MFA Proxy)     │◀────│     Vault       │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │  Notification    │
                        │  (Duo, Slack)    │
                        └──────────────────┘
```

## Core Components

### 1. HTTP Server (`internal/server`)

The HTTP server handles all API requests using the Gin framework.

**Endpoints:**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check endpoint |
| `/v1/secrets/request` | POST | Create access request |
| `/v1/secrets/{path}` | GET | Access approved secret |
| `/v1/requests/{id}/approve` | POST | Approve a request |
| `/v1/requests/{id}/deny` | POST | Deny a request |
| `/webhooks/{provider}` | POST | Provider callbacks |
| `/metrics` | GET | Prometheus metrics |

**Key Features:**
- TLS termination with optional mTLS
- Rate limiting per client
- CORS support
- Request logging and tracing

### 2. Authentication Layer (`internal/auth`)

Supports multiple authentication methods:

**API Key Authentication:**
```go
type APIKeyAuth struct {
    Keys map[string]*APIKey
}

type APIKey struct {
    Key          string
    ClientID     string
    AllowedPaths []string
    RateLimit    int
}
```

**mTLS Authentication:**
```go
type MTLSAuth struct {
    ClientCerts map[string]*ClientCert
}

type ClientCert struct {
    SubjectCN    string
    ClientID     string
    AllowedPaths []string
}
```

The auth middleware:
1. Extracts credentials from request headers or TLS state
2. Validates against configured keys/certificates
3. Checks path permissions
4. Applies rate limiting
5. Sets client context for downstream handlers

### 3. Provider Interface (`spec/provider_interface.go`)

Abstract interface for secret providers:

```go
type Provider interface {
    Name() string
    Configure(config map[string]interface{}) error
    Get(ctx context.Context, path string) (*Secret, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Health(ctx context.Context) error
    Close() error
}
```

**Implementations:**
- **1Password Connect**: HTTP client for 1Password Connect API
- **HashiCorp Vault**: Client using Vault Go SDK
- **AWS Secrets Manager**: AWS SDK integration
- **Local File**: Encrypted file storage with age/GPG

### 4. Approval Workflow Engine (`internal/server/approval.go`)

Manages the lifecycle of secret access requests:

```go
type Request struct {
    ID                string
    ClientID          string
    SecretPath        string
    Reason            string
    RequestedDuration time.Duration
    Status            RequestStatus
    CreatedAt         time.Time
    ExpiresAt         time.Time
    Approvals         []Approval
}

type Approval struct {
    Approver  string
    Approved  bool
    Reason    string
    Timestamp time.Time
}
```

**State Machine:**

```
┌─────────┐    Create     ┌──────────────┐
│  Start  │──────────────▶│    PENDING   │
└─────────┘               └──────────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
       ┌──────────┐    ┌──────────┐    ┌──────────┐
       │ APPROVED │    │  DENIED  │    │ EXPIRED  │
       └──────────┘    └──────────┘    └──────────┘
              │
              ▼
       ┌──────────┐
       │ ACCESSED │
       └──────────┘
```

### 5. Notification Manager (`spec/notification_interface.go`)

Interface for notification providers:

```go
type Provider interface {
    Name() string
    Configure(config map[string]interface{}) error
    Send(ctx context.Context, notification *Notification) (*Result, error)
    SupportsInteractive() bool
    Health(ctx context.Context) error
}
```

**Notification Flow:**

1. Request created
2. Policy evaluation determines approvers
3. Notification sent via configured providers
4. Provider delivers to approver(s)
5. Approver interacts (approve/deny)
6. Callback updates request state
7. Client notified of outcome

### 6. Policy Engine (`internal/server/policy.go`)

Evaluates access requests against configured policies:

```go
type Policy struct {
    RequiredApprovals      int
    MaxDuration            time.Duration
    RequestTimeout         time.Duration
    AllowSelfApproval      bool
    RequireJustification   bool
    MinJustificationLength int
    ApproverGroups         []string
    NotificationChannels   []string
}

type PolicyEngine struct {
    Default   *Policy
    Overrides []PathPolicy
}

type PathPolicy struct {
    PathPattern string
    Policy      *Policy
}
```

**Evaluation Algorithm:**

```go
func (e *PolicyEngine) Evaluate(path string) *Policy {
    // Start with default policy
    result := e.Default
    
    // Find matching overrides (most specific wins)
    for _, override := range e.Overrides {
        if match(path, override.PathPattern) {
            result = merge(result, override.Policy)
        }
    }
    
    return result
}
```

### 7. Storage Layer (`internal/storage`)

Persists request state and audit logs:

**SQLite (Development):**
```go
type SQLiteStorage struct {
    db *sql.DB
}
```

**PostgreSQL (Production):**
```go
type PostgresStorage struct {
    db *gorm.DB
}
```

**Schema:**

```sql
CREATE TABLE requests (
    id UUID PRIMARY KEY,
    client_id VARCHAR(255) NOT NULL,
    secret_path VARCHAR(1024) NOT NULL,
    reason TEXT,
    requested_duration INTERVAL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    approved_at TIMESTAMP,
    accessed_at TIMESTAMP
);

CREATE TABLE approvals (
    id UUID PRIMARY KEY,
    request_id UUID REFERENCES requests(id),
    approver VARCHAR(255) NOT NULL,
    approved BOOLEAN NOT NULL,
    reason TEXT,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_requests_status ON requests(status);
CREATE INDEX idx_requests_client ON requests(client_id);
CREATE INDEX idx_requests_expires ON requests(expires_at);
```

### 8. Audit Logger (`internal/audit`)

Records all significant events:

```go
type AuditLogger interface {
    Log(event *AuditEvent) error
}

type AuditEvent struct {
    Timestamp   time.Time
    EventType   EventType
    ClientID    string
    RequestID   string
    SecretPath  string
    Approver    string
    Success     bool
    Error       string
    Metadata    map[string]string
}
```

**Destinations:**
- File (with rotation)
- Syslog
- Webhook (SIEM integration)
- Database

### 9. Configuration (`internal/config`)

Configuration management with environment variable support:

```go
type Config struct {
    Server       ServerConfig
    Auth         AuthConfig
    Providers    ProvidersConfig
    Notifications NotificationsConfig
    Policies     PoliciesConfig
    Storage      StorageConfig
    Audit        AuditConfig
    Metrics      MetricsConfig
    Health       HealthConfig
    BreakGlass   BreakGlassConfig
}
```

**Priority (highest to lowest):**
1. Environment variables (`TRAVERSE_*`)
2. Configuration file
3. Default values

## Data Flow

### Secret Access Request Flow

```
1. Client POST /v1/secrets/request
   └─▶ Auth middleware validates credentials
       └─▶ Request handler receives request
           └─▶ Validate request parameters
               └─▶ Policy engine evaluates path
                   └─▶ Create request in database
                       └─▶ Send notifications to approvers
                           └─▶ Return request ID to client

2. Approver receives notification
   └─▶ Interacts via Duo/Slack/etc.
       └─▶ Provider sends callback to Traverse
           └─▶ Update request status
               └─▶ Notify client (if waiting)

3. Client GET /v1/secrets/{path}
   └─▶ Auth middleware validates
       └─▶ Check request is approved and not expired
           └─▶ Fetch secret from provider
               └─▶ Return secret to client
               └─▶ Log access to audit
```

## Security Architecture

### Authentication

- **API Keys**: SHA-256 hashed in memory, compared using constant-time comparison
- **mTLS**: Client certificate validation with configurable CAs
- **Token Rotation**: Automatic refresh for provider tokens (Vault, 1Password)

### Authorization

- **Path-based**: Clients have allow-lists of secret paths (glob patterns)
- **Time-bound**: Access grants expire after approved duration
- **Approval-required**: All access requires explicit approval

### Data Protection

- **In Transit**: TLS 1.3 with strong cipher suites
- **At Rest**: Secrets not stored by Traverse (fetched on-demand from providers)
- **Audit Logs**: Tamper-evident with signed log entries

### Threat Model

| Threat | Mitigation |
|--------|-----------|
| Unauthorized access | mTLS/API keys + approval workflows |
| Eavesdropping | TLS encryption |
| Replay attacks | Request expiration + nonces |
| Privilege escalation | Path-based permissions |
| Audit tampering | Immutable audit logs |
| Secret exposure | Secrets never stored, only proxied |

## Scalability

### Horizontal Scaling

Traverse is stateless except for the database:

```
                    ┌─────────────┐
                    │   Load      │
                    │  Balancer   │
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
           ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │  Traverse   │ │  Traverse   │ │  Traverse   │
    │   Pod 1     │ │   Pod 2     │ │   Pod N     │
    └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
           │               │               │
           └───────────────┼───────────────┘
                           ▼
                    ┌─────────────┐
                    │ PostgreSQL  │
                    │   Cluster   │
                    └─────────────┘
```

### Caching

- **Provider responses**: Cached with TTL (configurable)
- **Policy evaluations**: Cached per path
- **Health checks**: Cached to reduce provider load

### Connection Pooling

- Database connections: Pooled with configurable limits
- Provider connections: Reused via HTTP keep-alive
- Notification channels: Connection pooling per provider

## Deployment Patterns

### Docker Compose (Single Node)

Best for: Development, small teams

```yaml
services:
  traverse:
    image: funkymonkeymonk/traverse:latest
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
    depends_on:
      - postgres
      - op-connect
```

### Kubernetes (Production)

Best for: High availability, auto-scaling

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traverse
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: traverse
        image: funkymonkeymonk/traverse:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: traverse-config
```

### NixOS Module

Best for: Reproducible infrastructure, Nix ecosystems

```nix
services.traverse = {
  enable = true;
  settings.server.port = 8080;
};
```

## Monitoring

### Health Checks

```
GET /health

{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "version": "1.0.0",
  "checks": {
    "database": {"status": "healthy", "latency_ms": 5},
    "providers": {
      "1password": {"status": "healthy"},
      "vault": {"status": "unhealthy", "error": "timeout"}
    }
  }
}
```

### Metrics (Prometheus)

```
# Request rate
traverse_requests_total{status="pending",provider="1password"}

# Latency
traverse_request_duration_seconds_bucket{le="0.1"}

# Active requests
traverse_active_requests

# Provider health
traverse_provider_health{provider="1password"}

# Audit events
traverse_audit_events_total{event_type="REQUEST_APPROVED"}
```

### Alerting Rules

```yaml
groups:
- name: traverse
  rules:
  - alert: TraverseHighErrorRate
    expr: rate(traverse_requests_total{status="error"}[5m]) > 0.1
    for: 5m
    
  - alert: TraverseProviderDown
    expr: traverse_provider_health == 0
    for: 1m
    
  - alert: TraverseHighPendingRequests
    expr: traverse_active_requests > 100
    for: 10m
```

## Configuration Hot-Reload

Traverse supports configuration hot-reload without restart:

```bash
# Send SIGHUP to reload config
kill -HUP $(pgrep traverse)

# Or via API
POST /v1/admin/reload
```

**What reloads:**
- Policies
- Rate limits
- Notification providers
- Audit destinations

**What requires restart:**
- Server port/bind address
- TLS certificates
- Storage backend

## Error Handling

### Provider Errors

```go
func (s *Server) handleProviderError(err error) *APIError {
    switch {
    case errors.Is(err, provider.ErrSecretNotFound):
        return &APIError{
            Code:    "SECRET_NOT_FOUND",
            Message: "Secret not found",
            Status:  http.StatusNotFound,
        }
    case errors.Is(err, provider.ErrAccessDenied):
        return &APIError{
            Code:    "ACCESS_DENIED",
            Message: "Access denied by provider",
            Status:  http.StatusForbidden,
        }
    default:
        return &APIError{
            Code:    "PROVIDER_ERROR",
            Message: "Provider error",
            Status:  http.StatusServiceUnavailable,
        }
    }
}
```

### Circuit Breaker

For provider resilience:

```go
type CircuitBreaker struct {
    failureThreshold int
    successThreshold int
    timeout          time.Duration
    state            State
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == Open {
        return ErrCircuitOpen
    }
    
    err := fn()
    cb.recordResult(err)
    return err
}
```

## Future Enhancements

Planned features:

1. **Federation**: Multi-region secret replication
2. **Caching Layer**: Redis for high-throughput scenarios
3. **WebSocket Support**: Real-time request updates
4. **GraphQL API**: More flexible data queries
5. **OPA Integration**: Open Policy Agent for complex policies
6. **HSM Support**: Hardware security module integration
