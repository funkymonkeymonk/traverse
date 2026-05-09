# Security Best Practices

Guidelines for securing your Traverse deployment.

## Defense in Depth

Traverse uses multiple layers of security. Follow these practices to maximize protection.

## 1. Network Security

### Use TLS Everywhere

```yaml
server:
  tls:
    cert_file: "/etc/traverse/certs/server.crt"
    key_file: "/etc/traverse/certs/server.key"
    min_version: "1.3"  # Require TLS 1.3
```

**Certificate Management:**
- Use certificates from a trusted CA (Let's Encrypt, internal CA)
- Set up automatic renewal
- Monitor certificate expiration
- Use strong cipher suites

### Enable mTLS for Sensitive Environments

```yaml
server:
  tls:
    client_ca_file: "/etc/traverse/certs/ca.crt"
    client_auth: "require_and_verify"
```

**Benefits:**
- Client certificate authentication
- Prevents credential theft
- Automatic certificate rotation

### Network Segmentation

```
Internet
    │
    ▼
[Load Balancer] ──► [Traverse]
                          │
                          ▼
              [Internal Network]
                   │        │
                   ▼        ▼
              [Vault]  [1Password Connect]
```

- Place Traverse in DMZ
- Keep secret providers in internal network
- Use firewall rules to restrict access

## 2. Authentication

### Prefer mTLS Over API Keys

```yaml
# Production: mTLS
auth:
  type: "mtls"
  client_certificates:
    - subject_cn: "ci-runner.company.com"
      client_id: "ci-runner"

# Development: API keys
auth:
  type: "api_key"
  api_keys:
    - key: "${API_KEY}"  # Never hardcode!
```

### API Key Security

**Do:**
- Generate strong keys: `openssl rand -hex 32`
- Rotate keys every 90 days
- Store in secret management (not in git)
- Use different keys per client
- Restrict key permissions by path

**Don't:**
- Commit keys to version control
- Share keys between clients
- Use weak or short keys
- Log keys in plaintext

### Certificate Rotation

Automated rotation script:

```bash
#!/bin/bash
# rotate-certs.sh

# Generate new certificate
openssl req -new -newkey rsa:4096 -days 90 -nodes -x509 \
  -subj "/CN=traverse-client" \
  -keyout client-new.key \
  -out client-new.crt

# Update Traverse with new CA/cert
kubectl create secret tls traverse-tls-new \
  --cert=client-new.crt \
  --key=client-new.key \
  --dry-run=client -o yaml | kubectl apply -f -

# Graceful reload
kubectl rollout restart deployment traverse

# Update clients (gradual)
# - Deploy new cert to subset of clients
# - Monitor for errors
# - Roll out to all clients
# - Revoke old cert
```

## 3. Authorization

### Principle of Least Privilege

```yaml
auth:
  api_keys:
    # CI runner - limited access
    - key: "${CI_API_KEY}"
      client_id: "ci-runner"
      allowed_paths: 
        - "ci/*"
        - "shared/*"
      rate_limit: 10
      
    # Deployment bot - broader access
    - key: "${DEPLOY_API_KEY}"
      client_id: "deploy-bot"
      allowed_paths:
        - "prod/api-keys/*"
        - "prod/database/*"
      rate_limit: 5
```

### Path-Based Access Control

```yaml
# Never allow wildcard access to all paths
# ❌ Bad
allowed_paths: ["*"]

# ✅ Good
allowed_paths:
  - "prod/api-keys/*"
  - "prod/database/credentials"
  - "shared/*"
```

### Rate Limiting

```yaml
server:
  rate_limit:
    requests_per_minute: 60
    burst: 10

# Stricter limits for sensitive paths
policies:
  overrides:
    - path_pattern: "prod/admin/*"
      rate_limit:
        requests_per_minute: 10
```

## 4. Approval Workflows

### Require Multiple Approvers for Critical Paths

```yaml
policies:
  default:
    required_approvals: 1
    
  overrides:
    # Production requires dual control
    - path_pattern: "prod/*"
      required_approvals: 2
      approver_groups: ["ops-oncall", "security-team"]
      
    # Admin access requires 3 approvers
    - path_pattern: "*/admin"
      required_approvals: 3
      approver_groups: ["senior-staff"]
```

### Time-Bound Access

```yaml
policies:
  default:
    # Short default duration
    max_duration: "1h"
    
  overrides:
    # Even shorter for sensitive data
    - path_pattern: "prod/database/*"
      max_duration: "15m"
```

### Justification Requirements

```yaml
policies:
  default:
    require_justification: true
    min_justification_length: 20
    
    # Stronger requirements for production
  overrides:
    - path_pattern: "prod/*"
      require_justification: true
      min_justification_length: 50
```

## 5. Audit Logging

### Comprehensive Audit Trail

```yaml
audit:
  type: "multi"
  destinations:
    # Local file for immediate investigation
    - type: "file"
      file:
        path: "/var/log/traverse/audit.log"
        max_size: 100
        
    # SIEM integration for security monitoring
    - type: "webhook"
      webhook:
        url: "https://siem.company.com/api/events"
        headers:
          Authorization: "Bearer ${SIEM_TOKEN}"
        filter_events:
          - "REQUEST_CREATED"
          - "REQUEST_APPROVED"
          - "SECRET_ACCESSED"
          - "AUTH_FAILURE"
```

### Immutable Audit Logs

```bash
# Use append-only filesystem attributes
chattr +a /var/log/traverse/audit.log

# Ship logs to external SIEM immediately
# Use log forwarding agents (Filebeat, Fluentd)

# Regular log rotation with encryption
logrotate:
  compress: true
  dateext: true
  olddir: /var/log/traverse/archived
```

### Audit Review

```bash
# Check for suspicious patterns
grep "AUTH_FAILURE" /var/log/traverse/audit.log | jq -r '.ip' | sort | uniq -c | sort -rn

# Find unusual access times
jq 'select(.timestamp | contains("T02:"))' /var/log/traverse/audit.log

# Detect privilege escalation attempts
grep "POLICY_VIOLATION" /var/log/traverse/audit.log
```

## 6. Secret Management

### Provider Security

**1Password:**
- Use dedicated vault for Traverse
- Limit token to read-only access
- Rotate tokens every 90 days
- Monitor token usage

**Vault:**
- Use AppRole with short TTLs
- Enable audit logging in Vault
- Use separate policies per environment
- Enable response wrapping

**AWS Secrets Manager:**
- Use IAM roles (not access keys)
- Enable CloudTrail logging
- Use VPC endpoints
- Rotate secrets regularly

### Encryption at Rest

Local file provider:

```yaml
providers:
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"
    encryption:
      type: "age"
      recipient: "${AGE_PUBLIC_KEY}"
```

**Key Management:**
- Store private keys in HSM or KMS
- Never commit keys to git
- Use separate keys per environment
- Implement key rotation procedures

## 7. Monitoring and Alerting

### Security Metrics

```yaml
# Alert on suspicious activity
groups:
- name: traverse-security
  rules:
  # High rate of auth failures
  - alert: TraverseAuthFailures
    expr: rate(traverse_audit_events_total{event_type="AUTH_FAILURE"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High rate of authentication failures"
      
  # Access outside business hours
  - alert: TraverseAfterHoursAccess
    expr: hour() < 8 or hour() > 18
    for: 0m
    labels:
      severity: info
      
  # Break-glass usage
  - alert: TraverseBreakGlassUsed
    expr: traverse_audit_events_total{event_type="BREAK_GLASS_ACCESS"} > 0
    for: 0m
    labels:
      severity: critical
```

### Health Monitoring

```bash
# Check provider health
curl http://localhost:8080/health | jq '.checks.providers'

# Monitor certificate expiration
openssl x509 -in server.crt -noout -dates

# Check for errors
journalctl -u traverse -p err -f
```

## 8. Hardening

### Container Security

```dockerfile
# Use distroless or minimal base image
FROM gcr.io/distroless/static:nonroot

# Run as non-root
USER 65532:65532

# Read-only root filesystem
readOnlyRootFilesystem: true

# Drop all capabilities
securityContext:
  capabilities:
    drop:
    - ALL
```

### Kubernetes Security

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: traverse
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          capabilities:
            drop:
            - ALL
```

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: traverse
spec:
  podSelector:
    matchLabels:
      app: traverse
  policyTypes:
  - Ingress
  - Egress
  ingress:
  # Only allow from ingress controller
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  # Only allow to database and secret providers
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - podSelector:
        matchLabels:
          app: op-connect
    ports:
    - protocol: TCP
      port: 8080
```

## 9. Backup and Disaster Recovery

### Database Backups

```bash
# Automated backup script
#!/bin/bash
pg_dump -h localhost -U traverse traverse | \
  gzip | \
  openssl enc -aes-256-cbc -salt -pass pass:"${BACKUP_KEY}" \
  > "/backups/traverse-$(date +%Y%m%d-%H%M%S).sql.gz.enc"

# Upload to secure storage
aws s3 cp "$BACKUP_FILE" s3://secure-backups/traverse/
```

### Configuration Backups

```bash
# Backup config (without secrets)
grep -v "password\|secret\|key" /etc/traverse/config.yaml > config-sanitized.yaml

# Store in version control (sanitized)
git add config-sanitized.yaml
```

### Disaster Recovery Plan

1. **Document recovery procedures**
2. **Test restores regularly**
3. **Keep backups in multiple locations**
4. **Maintain offline backups for critical data**
5. **Document RTO/RPO targets**

## 10. Incident Response

### Break-Glass Access

```yaml
break_glass:
  enabled: true
  require_hardware_token: true
  audit_severity: "critical"
  notify:
    - "security@company.com"
    - "#security-alerts"
```

**Usage:**
```bash
# Emergency access
curl -X POST http://localhost:8080/v1/secrets/break-glass \
  -H "Authorization: Bearer ${BREAK_GLASS_TOKEN}" \
  -H "X-Hardware-Token: ${YUBIKEY_OTP}"
```

### Response Checklist

**Suspicious Activity Detected:**
1. [ ] Revoke affected API keys
2. [ ] Review audit logs
3. [ ] Check for unauthorized access
4. [ ] Rotate all credentials
5. [ ] Notify security team
6. [ ] Document incident

**Compromised Secret:**
1. [ ] Revoke secret in provider (1Password, Vault)
2. [ ] Rotate to new secret
3. [ ] Review audit logs for access
4. [ ] Notify affected systems
5. [ ] Update incident report

## Security Checklist

Before deploying to production:

- [ ] TLS enabled with valid certificates
- [ ] mTLS configured for sensitive clients
- [ ] API keys rotated and stored securely
- [ ] Path-based access controls configured
- [ ] Approval policies require multiple approvers for production
- [ ] Audit logging enabled with SIEM integration
- [ ] Rate limiting configured
- [ ] Health monitoring and alerting enabled
- [ ] Container security context configured
- [ ] Network policies in place
- [ ] Backup and recovery tested
- [ ] Incident response plan documented
- [ ] Security training completed for approvers

## Compliance

Traverse can help meet compliance requirements:

| Requirement | Traverse Feature |
|-------------|------------------|
| Access control | API keys, mTLS, path permissions |
| Audit trail | Comprehensive audit logging |
| Separation of duties | Multi-approver workflows |
| Least privilege | Path-based permissions |
| Encryption | TLS in transit, provider encryption at rest |
| Monitoring | Prometheus metrics, health checks |

For specific compliance frameworks (SOC 2, PCI DSS, HIPAA), contact your security team for additional requirements.
