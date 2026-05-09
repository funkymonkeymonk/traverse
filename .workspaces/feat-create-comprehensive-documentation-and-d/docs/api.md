# API Reference

Complete reference for the Traverse REST API.

## Base URL

```
https://traverse.company.com
```

All API requests must include an `Authorization` header:

```
Authorization: Bearer YOUR_API_KEY
```

## Authentication

Traverse supports two authentication methods:

### API Key

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  https://traverse.company.com/v1/secrets/request
```

### mTLS (Mutual TLS)

When mTLS is enabled, provide a client certificate:

```bash
curl --cert client.crt --key client.key \
  https://traverse.company.com/v1/secrets/request
```

## Endpoints

### Health Check

Check the health status of the Traverse server.

```http
GET /health
```

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "version": "1.0.0",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5
    },
    "providers": {
      "1password": {
        "status": "healthy"
      }
    }
  }
}
```

**Status Codes:**
- `200` - Healthy
- `503` - Unhealthy (one or more checks failing)

---

### Create Access Request

Request access to a secret.

```http
POST /v1/secrets/request
Content-Type: application/json
```

**Request Body:**

```json
{
  "secret_path": "prod/database/password",
  "reason": "Database migration for v2.0",
  "duration": "30m",
  "approvers": ["admin@company.com"]
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secret_path` | string | Yes | Path to the secret |
| `reason` | string | Yes | Justification for access |
| `duration` | string | No | Access duration (default: policy max) |
| `approvers` | array | No | Specific approvers to notify |

**Response (201 Created):**

```json
{
  "request_id": "req_abc123def456",
  "status": "pending_approval",
  "message": "Request sent to approvers via Slack",
  "expires_at": "2024-01-15T10:30:00Z",
  "estimated_wait": "5m",
  "approval_url": "https://traverse.company.com/v1/requests/req_abc123def456/status"
}
```

**Status Codes:**
- `201` - Request created
- `400` - Invalid request
- `401` - Unauthorized
- `403` - Forbidden (path not allowed)
- `429` - Rate limited

**Error Response:**

```json
{
  "error": {
    "code": "PATH_NOT_ALLOWED",
    "message": "Client does not have access to path 'prod/admin/super-secret'",
    "details": {
      "path": "prod/admin/super-secret",
      "allowed_paths": ["prod/*", "shared/*"]
    }
  }
}
```

---

### Get Request Status

Check the status of an access request.

```http
GET /v1/requests/{request_id}
```

**Response:**

```json
{
  "request_id": "req_abc123def456",
  "status": "pending_approval",
  "client_id": "deployment-bot",
  "secret_path": "prod/database/password",
  "reason": "Database migration for v2.0",
  "created_at": "2024-01-15T10:00:00Z",
  "expires_at": "2024-01-15T10:05:00Z",
  "approvals_required": 1,
  "approvals_received": 0,
  "approvers": [
    {
      "identity": "admin@company.com",
      "name": "Admin User",
      "status": "pending"
    }
  ]
}
```

**Status Codes:**
- `200` - Success
- `404` - Request not found

---

### Approve Request

Approve a pending access request.

```http
POST /v1/requests/{request_id}/approve
Content-Type: application/json
```

**Request Body:**

```json
{
  "reason": "Approved for deployment"
}
```

**Response:**

```json
{
  "request_id": "req_abc123def456",
  "status": "approved",
  "approved_by": "admin@company.com",
  "approved_at": "2024-01-15T10:02:00Z",
  "expires_at": "2024-01-15T10:32:00Z"
}
```

**Status Codes:**
- `200` - Approved
- `400` - Already approved/denied/expired
- `403` - Not an authorized approver
- `404` - Request not found

---

### Deny Request

Deny a pending access request.

```http
POST /v1/requests/{request_id}/deny
Content-Type: application/json
```

**Request Body:**

```json
{
  "reason": "Not authorized for this secret"
}
```

**Response:**

```json
{
  "request_id": "req_abc123def456",
  "status": "denied",
  "denied_by": "admin@company.com",
  "denied_at": "2024-01-15T10:02:00Z",
  "reason": "Not authorized for this secret"
}
```

**Status Codes:**
- `200` - Denied
- `400` - Already approved/denied/expired
- `403` - Not an authorized approver
- `404` - Request not found

---

### Access Secret

Retrieve a secret (requires approved request).

```http
GET /v1/secrets/{path}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | URL-encoded secret path |

**Response:**

```json
{
  "path": "prod/database/password",
  "values": {
    "host": "db.company.com",
    "port": "5432",
    "username": "app_user",
    "password": "super_secret_password_123"
  },
  "metadata": {
    "version": "1",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-10T00:00:00Z"
  },
  "expires_at": "2024-01-15T10:32:00Z",
  "access_id": "acc_xyz789"
}
```

**Status Codes:**
- `200` - Success
- `401` - No approved request
- `403` - Request expired
- `404` - Secret not found

**Error Response:**

```json
{
  "error": {
    "code": "NO_APPROVED_REQUEST",
    "message": "No approved request found for this secret",
    "details": {
      "secret_path": "prod/database/password",
      "action": "Create a request first: POST /v1/secrets/request"
    }
  }
}
```

---

### List Secrets

List available secrets (if provider supports listing).

```http
GET /v1/secrets?prefix={prefix}
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prefix` | string | No | Path prefix to filter by |

**Response:**

```json
{
  "secrets": [
    "prod/api-keys/stripe",
    "prod/api-keys/sendgrid",
    "prod/database/password"
  ],
  "count": 3,
  "prefix": "prod"
}
```

**Status Codes:**
- `200` - Success
- `501` - Provider does not support listing

---

### List My Requests

List requests made by the current client.

```http
GET /v1/requests?status={status}&limit={limit}&offset={offset}
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `status` | string | No | Filter by status (pending, approved, denied, expired) |
| `limit` | int | No | Number of results (default: 20, max: 100) |
| `offset` | int | No | Pagination offset |

**Response:**

```json
{
  "requests": [
    {
      "request_id": "req_abc123def456",
      "status": "approved",
      "secret_path": "prod/database/password",
      "reason": "Database migration",
      "created_at": "2024-01-15T10:00:00Z",
      "expires_at": "2024-01-15T10:32:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

---

### Webhook Callbacks

Endpoints for notification provider callbacks.

#### Slack Interactive

```http
POST /webhooks/slack
Content-Type: application/x-www-form-urlencoded
```

Slack sends interaction payloads here when users click approval buttons.

#### Duo Callback

```http
POST /webhooks/duo
```

Duo authentication responses are sent here.

---

### Admin Endpoints

Require admin API key.

#### Reload Configuration

```http
POST /v1/admin/reload
```

Hot-reload configuration without restart.

**Response:**

```json
{
  "status": "success",
  "message": "Configuration reloaded",
  "timestamp": "2024-01-15T10:00:00Z"
}
```

#### Test Notification

```http
POST /v1/admin/notifications/test
Content-Type: application/json
```

**Request Body:**

```json
{
  "provider": "slack",
  "recipient": "#test-channel"
}
```

**Response:**

```json
{
  "status": "sent",
  "provider": "slack",
  "recipient": "#test-channel",
  "message_id": "1234567890.123456"
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Request body is invalid |
| `PATH_NOT_ALLOWED` | 403 | Client cannot access this path |
| `NO_APPROVED_REQUEST` | 401 | No approved request exists |
| `REQUEST_EXPIRED` | 403 | Approved request has expired |
| `SECRET_NOT_FOUND` | 404 | Secret does not exist |
| `REQUEST_NOT_FOUND` | 404 | Request ID not found |
| `NOT_APPROVER` | 403 | User is not authorized to approve |
| `RATE_LIMITED` | 429 | Too many requests |
| `PROVIDER_ERROR` | 503 | Secret provider is unavailable |
| `INTERNAL_ERROR` | 500 | Internal server error |

## Rate Limiting

API requests are rate-limited per client:

- **Default**: 60 requests per minute
- **Burst**: 10 requests

Rate limit headers are included in responses:

```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 58
X-RateLimit-Reset: 1705312800
```

When rate limited:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30

{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded. Try again in 30 seconds."
  }
}
```

## SDK Examples

### Python

```python
import requests

class TraverseClient:
    def __init__(self, base_url, api_key):
        self.base_url = base_url
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        }
    
    def request_secret(self, path, reason, duration="1h"):
        response = requests.post(
            f"{self.base_url}/v1/secrets/request",
            headers=self.headers,
            json={
                "secret_path": path,
                "reason": reason,
                "duration": duration
            }
        )
        response.raise_for_status()
        return response.json()
    
    def get_secret(self, path):
        response = requests.get(
            f"{self.base_url}/v1/secrets/{path}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

# Usage
client = TraverseClient("https://traverse.company.com", "your-api-key")
request = client.request_secret("prod/api-keys/stripe", "Payment processing")
print(f"Request ID: {request['request_id']}")
# Wait for approval...
secret = client.get_secret("prod/api-keys/stripe")
print(f"API Key: {secret['values']['api_key']}")
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type TraverseClient struct {
    BaseURL string
    APIKey  string
    Client  *http.Client
}

type SecretRequest struct {
    SecretPath string `json:"secret_path"`
    Reason     string `json:"reason"`
    Duration   string `json:"duration"`
}

func (c *TraverseClient) RequestSecret(path, reason, duration string) (*RequestResponse, error) {
    reqBody, _ := json.Marshal(SecretRequest{
        SecretPath: path,
        Reason:     reason,
        Duration:   duration,
    })
    
    req, _ := http.NewRequest("POST", c.BaseURL+"/v1/secrets/request", bytes.NewBuffer(reqBody))
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result RequestResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

// Usage
func main() {
    client := &TraverseClient{
        BaseURL: "https://traverse.company.com",
        APIKey:  "your-api-key",
        Client:  &http.Client{},
    }
    
    resp, _ := client.RequestSecret("prod/api-keys/stripe", "Payment processing", "1h")
    fmt.Printf("Request ID: %s\n", resp.RequestID)
}
```

### JavaScript/Node.js

```javascript
const axios = require('axios');

class TraverseClient {
  constructor(baseURL, apiKey) {
    this.client = axios.create({
      baseURL,
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json'
      }
    });
  }
  
  async requestSecret(path, reason, duration = '1h') {
    const response = await this.client.post('/v1/secrets/request', {
      secret_path: path,
      reason,
      duration
    });
    return response.data;
  }
  
  async getSecret(path) {
    const response = await this.client.get(`/v1/secrets/${encodeURIComponent(path)}`);
    return response.data;
  }
}

// Usage
const client = new TraverseClient('https://traverse.company.com', 'your-api-key');
const request = await client.requestSecret('prod/api-keys/stripe', 'Payment processing');
console.log(`Request ID: ${request.request_id}`);
```

## WebSocket API (Future)

Real-time updates are planned for a future release:

```javascript
const ws = new WebSocket('wss://traverse.company.com/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    action: 'subscribe',
    request_id: 'req_abc123def456'
  }));
};

ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log('Request update:', update);
};
```
