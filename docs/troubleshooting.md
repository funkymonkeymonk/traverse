# Troubleshooting Guide

Common issues and their solutions.

## Installation Issues

### Docker: Port Already in Use

**Error:**
```
bind: address already in use
```

**Solution:**
```bash
# Find process using port 8080
sudo lsof -i :8080

# Kill the process or change Traverse port
# Edit docker-compose.yml or config.yaml:
server:
  port: 8081  # Use different port
```

### Docker: Permission Denied on Volumes

**Error:**
```
permission denied: /var/lib/traverse
```

**Solution:**
```bash
# Fix ownership
sudo chown -R 1000:1000 /path/to/traverse/data

# Or run with user ID
services:
  traverse:
    user: "${UID}:${GID}"
```

### Binary: "command not found"

**Error:**
```
traverse: command not found
```

**Solution:**
```bash
# Check if in PATH
echo $PATH

# Add to PATH or use full path
export PATH=$PATH:/usr/local/bin

# Or run with full path
/usr/local/bin/traverse --help
```

## Configuration Issues

### Config File Not Found

**Error:**
```
config file not found: /etc/traverse/config.yaml
```

**Solution:**
```bash
# Check file exists
ls -la /etc/traverse/config.yaml

# Create directory
sudo mkdir -p /etc/traverse

# Specify config path
traverse --config /path/to/config.yaml

# Validate YAML syntax
cat /etc/traverse/config.yaml | yq
```

### Invalid Configuration

**Error:**
```
failed to parse config: yaml: line 15: did not find expected key
```

**Solution:**
```bash
# Validate YAML
yq /etc/traverse/config.yaml

# Common issues:
# - Tabs instead of spaces (use 2 spaces)
# - Missing quotes around strings with special characters
# - Incorrect indentation
```

### Environment Variables Not Working

**Error:**
Variables not substituted in configuration.

**Solution:**
```bash
# Check variable is set
echo $TRAVERSE_SERVER_PORT

# Export variable
export TRAVERSE_SERVER_PORT=8080

# Or use .env file
set -a
source /etc/traverse/environment
set +a
```

## Authentication Issues

### Invalid API Key

**Error:**
```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid API key"
  }
}
```

**Solution:**
```bash
# Check your API key
curl -H "Authorization: Bearer YOUR_KEY" http://localhost:8080/health

# Verify key in config
# Ensure no extra spaces or newlines
cat /etc/traverse/config.yaml | grep api_key

# Generate new key if needed
openssl rand -hex 32
```

### mTLS Certificate Issues

**Error:**
```
tls: bad certificate
```

**Solution:**
```bash
# Verify certificate
openssl x509 -in client.crt -text -noout

# Check certificate dates
openssl x509 -in client.crt -noout -dates

# Verify chain
openssl verify -CAfile ca.crt client.crt

# Test connection
openssl s_client -connect localhost:8443 -cert client.crt -key client.key -CAfile ca.crt
```

### Path Not Allowed

**Error:**
```json
{
  "error": {
    "code": "PATH_NOT_ALLOWED",
    "message": "Client 'ci-runner' cannot access path 'prod/admin/*'"
  }
}
```

**Solution:**
```bash
# Check client configuration
# Update allowed_paths in config.yaml:
auth:
  api_keys:
    - key: "your-key"
      client_id: "ci-runner"
      allowed_paths: ["prod/*", "shared/*"]  # Add needed paths
```

## Provider Issues

### 1Password Connect Not Working

**Error:**
```
failed to connect to 1Password: connection refused
```

**Solution:**
```bash
# Check Connect server is running
docker ps | grep op-connect

# Test connectivity
curl http://localhost:8081/health

# Verify token
echo $OP_CONNECT_TOKEN | head -c 20

# Check logs
docker logs op-connect-api
```

### Vault Authentication Failed

**Error:**
```
vault: permission denied
```

**Solution:**
```bash
# Test Vault access
export VAULT_ADDR=https://vault.internal:8200
export VAULT_TOKEN=your-token
vault kv get secret/api-keys/stripe

# Check token policies
vault token lookup

# Renew token if expired
vault token renew

# Regenerate AppRole secret if needed
vault write -f auth/approle/role/traverse/secret-id
```

### AWS Credentials Not Found

**Error:**
```
aws: NoCredentialProviders: no valid providers
```

**Solution:**
```bash
# Check AWS credentials
aws sts get-caller-identity

# Set environment variables
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1

# Or use IAM role (EC2/EKS)
# Ensure instance/service account has correct IAM role
```

## Notification Issues

### Slack Messages Not Sending

**Error:**
```
slack: channel_not_found
```

**Solution:**
```bash
# Invite bot to channel
# In Slack: /invite @Traverse

# Verify bot token
# Should start with xoxb-
echo $SLACK_BOT_TOKEN | head -c 10

# Check scopes
# Must have chat:write, im:write

# Test manually
curl -X POST https://slack.com/api/chat.postMessage \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel":"#test","text":"Test message"}'
```

### Duo Push Not Received

**Error:**
```
duo: user not found
```

**Solution:**
```bash
# Check Duo username matches
# In config, approver contact_info should match Duo username

# Verify Duo integration
# Check Integration Key, Secret Key, API Hostname

# Test Duo API
curl "https://api-xxxxxxxx.duosecurity.com/auth/v2/check" \
  -H "Date: $(date -u '+%a, %d %b %Y %H:%M:%S %Z')" \
  -H "Authorization: Basic $(echo -n 'KEY:SECRET' | base64)"
```

## Request Issues

### Request Expired Before Approval

**Error:**
```json
{
  "error": {
    "code": "REQUEST_EXPIRED",
    "message": "Request expired at 2024-01-15T10:05:00Z"
  }
}
```

**Solution:**
```yaml
# Increase request_timeout in policy
policies:
  default:
    request_timeout: "15m"  # Increase from default 5m
```

### No Approvers Notified

**Error:**
Requests stuck in pending state.

**Solution:**
```bash
# Check notification providers are configured
# Check approvers are mapped correctly

# Debug notifications
curl -X POST http://localhost:8080/v1/admin/notifications/test \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{"provider":"slack","recipient":"#test"}'
```

### Secret Access Denied After Approval

**Error:**
```
secret access denied despite approved request
```

**Solution:**
```bash
# Check request hasn't expired
# Check accessing correct path
# Check provider is healthy

# View request details
curl http://localhost:8080/v1/requests/req_xxx \
  -H "Authorization: Bearer API_KEY"
```

## Database Issues

### PostgreSQL Connection Failed

**Error:**
```
postgresql: connection refused
```

**Solution:**
```bash
# Check PostgreSQL is running
sudo systemctl status postgresql

# Test connection
psql -h localhost -U traverse -d traverse

# Check connection string
# Ensure host, port, database, user, password are correct

# Check pg_hba.conf allows connections
# host  traverse  traverse  0.0.0.0/0  md5
```

### SQLite Permission Denied

**Error:**
```
sqlite: unable to open database file
```

**Solution:**
```bash
# Fix ownership
sudo chown traverse:traverse /var/lib/traverse/traverse.db

# Or use different path
storage:
  sqlite:
    path: "/home/user/traverse.db"
```

## Performance Issues

### High Memory Usage

**Solution:**
```yaml
# Reduce connection pool size
storage:
  postgresql:
    max_connections: 10  # Reduce from 20

# Enable caching
providers:
  1password:
    cache_ttl: "5m"  # Cache secrets for 5 minutes
```

### Slow Secret Retrieval

**Solution:**
```yaml
# Increase provider timeout
providers:
  1password:
    timeout: "30s"  # Increase from 10s

# Enable retries
    retry:
      max_retries: 5
      backoff: "exponential"
```

### Too Many Open Files

**Error:**
```
too many open files
```

**Solution:**
```bash
# Increase file descriptor limit
ulimit -n 4096

# Or in systemd service
[Service]
LimitNOFILE=4096
```

## Log Analysis

### Enable Debug Logging

```yaml
# config.yaml
logging:
  level: "debug"  # debug, info, warn, error
  format: "json"  # json, text
```

### Common Log Patterns

**Successful Request:**
```json
{
  "level": "info",
  "msg": "secret accessed",
  "request_id": "req_xxx",
  "client_id": "ci-runner",
  "secret_path": "prod/api-keys/stripe",
  "duration_ms": 150
}
```

**Authentication Failure:**
```json
{
  "level": "warn",
  "msg": "authentication failed",
  "client_id": "unknown",
  "error": "invalid api key",
  "ip": "10.0.0.1"
}
```

**Provider Error:**
```json
{
  "level": "error",
  "msg": "provider request failed",
  "provider": "1password",
  "error": "connection timeout",
  "retry_count": 3
}
```

## Getting Help

If the issue persists:

1. **Check documentation**
   - [Configuration Reference](configuration.md)
   - [Provider Setup](providers.md)
   - [Notification Setup](notifications.md)

2. **Enable debug logging**
   ```yaml
   logging:
     level: "debug"
   ```

3. **Gather information**
   ```bash
   # Version
   traverse --version
   
   # Config (sanitized)
   traverse --config /etc/traverse/config.yaml --validate
   
   # Health check
   curl http://localhost:8080/health
   
   # Logs
   journalctl -u traverse -n 100
   ```

4. **Create an issue**
   - GitHub: https://github.com/funkymonkeymonk/traverse/issues
   - Include logs, config (redact secrets), and reproduction steps
