# Sentinel API Specification

## Base URL
```
https://traverse.internal:8080/v1
```

## Authentication

### API Key Authentication
```
Authorization: Bearer traverse_api_xxxxxxxxxxxxxxxx
```

### mTLS Authentication
Clients present client certificates. Server validates against CA.

## Error Responses

All errors follow RFC 7807 (Problem Details):

```json
{
  "type": "https://traverse.internal/errors/invalid-request",
  "title": "Invalid Request",
  "status": 400,
  "detail": "The secret path 'prod/ api-key' contains invalid characters",
  "instance": "/v1/secrets/request",
  "request_id": "req_trace_abc123"
}
```

## Endpoints

### 1. Request Secret

Submit a request to access a secret. Requires approval before access is granted.

**Endpoint:** `POST /secrets/request`

**Authentication:** Required

**Request Body:**
```json
{
  "secret_path": "string (required)",
  "reason": "string (required)",
  "client_id": "string (optional, defaults to auth identity)",
  "requested_duration": "string (duration, default: 1h)",
  "notification_preferences": ["string"] (optional),
  "metadata": {
    "key": "value"
  }
}
```

**Validation Rules:**
- `secret_path`: Must match `^[a-zA-Z0-9_\-\/\.]+$`
- `reason`: Minimum 10 characters
- `requested_duration`: Must be <= policy max_duration
- `notification_preferences`: Must be subset of configured providers

**Response 202 Accepted:**
```json
{
  "request_id": "req_abc123def456",
  "status": "pending_approval",
  "message": "Request submitted. Approval notification sent to 2 approvers.",
  "poll_url": "/v1/requests/req_abc123def456/status",
  "websocket_url": "wss://traverse.internal/v1/requests/req_abc123def456/stream",
  "expires_at": "2026-05-08T10:05:00Z",
  "estimated_approval_time": "< 2 minutes"
}
```

**Response 400 Bad Request:**
```json
{
  "type": "https://traverse.internal/errors/invalid-request",
  "title": "Invalid Request",
  "status": 400,
  "detail": "Secret path 'prod/api-keys' is not accessible by this client",
  "violations": [
    {
      "field": "secret_path",
      "message": "Path not in allowed_paths list"
    }
  ]
}
```

**Response 429 Too Many Requests:**
```json
{
  "type": "https://traverse.internal/errors/rate-limited",
  "title": "Rate Limited",
  "status": 429,
  "detail": "Too many requests from client agent-001",
  "retry_after": 60
}
```

### 2. Get Request Status

Check the status of a pending or approved request.

**Endpoint:** `GET /requests/{request_id}/status`

**Authentication:** Required (must be requestor or approver)

**Response 200 OK (Pending):**
```json
{
  "request_id": "req_abc123def456",
  "status": "pending",
  "status_detail": "awaiting_approval",
  "client_id": "agent-001",
  "secret_path": "prod/api-keys/stripe",
  "reason": "Deploying payment feature",
  "created_at": "2026-05-08T10:00:00Z",
  "expires_at": "2026-05-08T10:05:00Z",
  "approvers_notified": [
    {
      "identity": "admin@company.com",
      "notified_at": "2026-05-08T10:00:01Z",
      "channel": "duo"
    },
    {
      "identity": "security@company.com", 
      "notified_at": "2026-05-08T10:00:02Z",
      "channel": "slack"
    }
  ],
  "approval_count": 0,
  "required_approvals": 1,
  "denial_count": 0
}
```

**Response 200 OK (Approved):**
```json
{
  "request_id": "req_abc123def456",
  "status": "approved",
  "status_detail": "access_granted",
  "client_id": "agent-001",
  "secret_path": "prod/api-keys/stripe",
  "reason": "Deploying payment feature",
  "created_at": "2026-05-08T10:00:00Z",
  "expires_at": "2026-05-08T10:05:00Z",
  "approved_at": "2026-05-08T10:01:30Z",
  "approved_by": [
    {
      "identity": "admin@company.com",
      "approved_at": "2026-05-08T10:01:30Z",
      "reason": "Approved for hotfix"
    }
  ],
  "token": "eyJhbGciOiJSUzI1NiIs...",
  "token_expires_at": "2026-05-08T11:01:30Z",
  "secret_url": "/v1/secrets/prod/api-keys/stripe"
}
```

**Response 200 OK (Denied):**
```json
{
  "request_id": "req_abc123def456",
  "status": "denied",
  "status_detail": "access_denied",
  "denied_at": "2026-05-08T10:02:00Z",
  "denied_by": [
    {
      "identity": "security@company.com",
      "denied_at": "2026-05-08T10:02:00Z",
      "reason": "Not authorized for production access"
    }
  ]
}
```

**Response 200 OK (Expired):**
```json
{
  "request_id": "req_abc123def456",
  "status": "expired",
  "status_detail": "request_timeout",
  "message": "Request expired before receiving required approvals"
}
```

### 3. Retrieve Secret

Fetch the actual secret value using an approved token.

**Endpoint:** `GET /secrets/{path}`

**Query Parameters:**
- `token` (required): JWT access token from approved request

**Authentication:** None (token carries authorization)

**Response 200 OK:**
```json
{
  "path": "prod/api-keys/stripe",
  "provider": "1password",
  "values": {
    "api_key": "sk_live_51H...xyz",
    "webhook_secret": "whsec_abc123...",
    "publishable_key": "pk_live_abc..."
  },
  "metadata": {
    "version": "3",
    "created_at": "2026-04-01T00:00:00Z",
    "updated_at": "2026-04-15T12:30:00Z",
    "tags": {
      "environment": "production",
      "service": "payments"
    }
  },
  "access": {
    "granted_at": "2026-05-08T10:01:30Z",
    "expires_at": "2026-05-08T11:01:30Z",
    "request_id": "req_abc123def456"
  }
}
```

**Response 401 Unauthorized:**
```json
{
  "type": "https://traverse.internal/errors/invalid-token",
  "title": "Invalid or Expired Token",
  "status": 401,
  "detail": "The provided token has expired or is invalid"
}
```

**Response 403 Forbidden:**
```json
{
  "type": "https://traverse.internal/errors/access-denied",
  "title": "Access Denied",
  "status": 403,
  "detail": "Token does not grant access to path 'prod/database'"
}
```

### 4. Approve Request

Approve a pending access request.

**Endpoint:** `POST /requests/{request_id}/approve`

**Authentication:** Required (must be authorized approver)

**Request Body:**
```json
{
  "reason": "string (optional)",
  "override_duration": "string (duration, optional)",
  "approved_paths": ["string"] (optional, restricts which paths are granted)
}
```

**Response 200 OK:**
```json
{
  "request_id": "req_abc123def456",
  "status": "approved",
  "message": "Request approved. Token issued.",
  "approved_at": "2026-05-08T10:01:30Z",
  "remaining_required_approvals": 0
}
```

**Response 409 Conflict (Already processed):**
```json
{
  "type": "https://traverse.internal/errors/request-processed",
  "title": "Request Already Processed",
  "status": 409,
  "detail": "Request req_abc123def456 was already approved by admin@company.com"
}
```

### 5. Deny Request

Deny a pending access request.

**Endpoint:** `POST /requests/{request_id}/deny`

**Authentication:** Required (must be authorized approver)

**Request Body:**
```json
{
  "reason": "string (required)"
}
```

**Response 200 OK:**
```json
{
  "request_id": "req_abc123def456",
  "status": "denied",
  "message": "Request denied.",
  "denied_at": "2026-05-08T10:02:00Z"
}
```

### 6. List Requests

List all requests with optional filtering.

**Endpoint:** `GET /requests`

**Query Parameters:**
- `status` (optional): `pending`, `approved`, `denied`, `expired`
- `client_id` (optional): Filter by client
- `secret_path` (optional): Filter by path prefix
- `from` (optional): Start time (ISO 8601)
- `to` (optional): End time (ISO 8601)
- `limit` (optional): Max results (default 50, max 1000)
- `offset` (optional): Pagination offset

**Response 200 OK:**
```json
{
  "requests": [
    {
      "request_id": "req_abc123def456",
      "status": "approved",
      "client_id": "agent-001",
      "secret_path": "prod/api-keys/stripe",
      "reason": "Deploying payment feature",
      "created_at": "2026-05-08T10:00:00Z",
      "resolved_at": "2026-05-08T10:01:30Z"
    }
  ],
  "pagination": {
    "total": 156,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

### 7. Revoke Token

Revoke an active token before expiration.

**Endpoint:** `POST /tokens/{token_id}/revoke`

**Authentication:** Required (admin or token owner)

**Request Body:**
```json
{
  "reason": "string (required)"
}
```

**Response 200 OK:**
```json
{
  "token_id": "tok_xyz789",
  "status": "revoked",
  "revoked_at": "2026-05-08T10:30:00Z",
  "reason": "Security incident - rotate credentials"
}
```

### 8. Health Check

Check service health and provider status.

**Endpoint:** `GET /health`

**Authentication:** None

**Response 200 OK:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "72h15m30s",
  "providers": [
    {
      "name": "1password",
      "status": "connected",
      "latency_ms": 45
    },
    {
      "name": "vault",
      "status": "connected", 
      "latency_ms": 12
    }
  ],
  "notifications": [
    {
      "provider": "duo",
      "status": "connected"
    },
    {
      "provider": "slack",
      "status": "connected"
    }
  ],
  "storage": {
    "type": "postgresql",
    "status": "connected",
    "pending_requests": 3
  }
}
```

### 9. WebSocket Stream

Real-time updates for request status changes.

**Endpoint:** `wss://traverse.internal/v1/requests/{request_id}/stream`

**Protocol:** WebSocket with JWT authentication in subprotocol or query param

**Messages:**

Server → Client:
```json
{
  "event": "status_changed",
  "timestamp": "2026-05-08T10:01:30Z",
  "data": {
    "status": "approved",
    "token": "eyJhbGc...",
    "token_expires_at": "2026-05-08T11:01:30Z"
  }
}
```

```json
{
  "event": "approver_notified",
  "timestamp": "2026-05-08T10:00:01Z",
  "data": {
    "approver": "admin@company.com",
    "channel": "duo"
  }
}
```

```json
{
  "event": "heartbeat",
  "timestamp": "2026-05-08T10:00:30Z"
}
```

## Webhook Events

Sentinel can POST to configured webhooks for real-time notifications:

**Headers:**
```
X-Sentinel-Event: request.approved
X-Sentinel-Signature: sha256=<hmac>
Content-Type: application/json
```

**Event Types:**
- `request.created` - New request submitted
- `request.approved` - Request approved
- `request.denied` - Request denied
- `request.expired` - Request timed out
- `token.issued` - Access token created
- `token.revoked` - Access token revoked
- `secret.accessed` - Secret retrieved

**Payload:**
```json
{
  "event": "request.approved",
  "timestamp": "2026-05-08T10:01:30Z",
  "request": {
    "request_id": "req_abc123def456",
    "client_id": "agent-001",
    "secret_path": "prod/api-keys/stripe",
    "approved_by": "admin@company.com"
  }
}
```

## Rate Limits

| Endpoint | Rate Limit |
|----------|-----------|
| POST /secrets/request | 10/min per client |
| GET /requests/{id}/status | 60/min per client |
| GET /secrets/{path} | 100/min per token |
| POST /requests/{id}/approve | 30/min per approver |
| GET /health | 100/min total |

Rate limit headers included in all responses:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 8
X-RateLimit-Reset: 1715164800
```

## Pagination

List endpoints support cursor-based pagination:

**Request:**
```
GET /requests?limit=50&cursor=eyJpZCI6MTIzNH0=
```

**Response:**
```json
{
  "data": [...],
  "pagination": {
    "next_cursor": "eyJpZCI6NTY3OH0=",
    "has_more": true
  }
}
```

## SDK Examples

### Go
```go
client := traverse.NewClient("https://traverse.internal:8080", apiKey)

// Request secret
req, err := client.RequestSecret(ctx, &traverse.Request{
    Path:     "prod/api-keys/stripe",
    Reason:   "Deploying feature",
    Duration: time.Hour,
})

// Poll for approval
secret, err := client.WaitForApproval(ctx, req.RequestID, 5*time.Minute)
```

### Python
```python
client = SentinelClient("https://traverse.internal:8080", api_key)

# Request with callback
request = client.request_secret(
    path="prod/api-keys/stripe",
    reason="Deploying feature",
    callback=lambda status: print(f"Status: {status}")
)

# Or use async/await
secret = await client.wait_for_approval(request.request_id)
```

### cURL
```bash
# Request
REQUEST_ID=$(curl -s -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"secret_path":"prod/api-keys/stripe","reason":"Deploy"}' \
  https://traverse.internal:8080/v1/secrets/request | jq -r .request_id)

# Poll status
curl -s -H "Authorization: Bearer $API_KEY" \
  https://traverse.internal:8080/v1/requests/$REQUEST_ID/status

# Get secret (after approval)
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://traverse.internal:8080/v1/secrets/prod/api-keys/stripe?token=$TOKEN"
```
