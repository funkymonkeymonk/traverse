# Kubernetes Deployment

Complete Kubernetes manifests for deploying Traverse in a production environment.

## Components

- **Traverse**: Main application (2 replicas for HA)
- **PostgreSQL**: Stateful database for request storage
- **1Password Connect**: For accessing 1Password secrets

## Prerequisites

- Kubernetes 1.24+
- kubectl configured
- TLS certificates (or cert-manager)
- Secrets configured

## Quick Start

```bash
# Create namespace
kubectl apply -f namespace.yaml

# Create ConfigMap with configuration
kubectl apply -f configmap.yaml

# Create secrets (edit values first!)
kubectl apply -f secret.yaml

# Deploy database
kubectl apply -f postgres.yaml

# Deploy Traverse
kubectl apply -f deployment.yaml

# Create services
kubectl apply -f service.yaml

# Optional: Create ingress
kubectl apply -f ingress.yaml
```

## Configuration

### 1. Secrets

Edit `secret.yaml` with your actual values:

```yaml
stringData:
  OP_CONNECT_TOKEN: "your-1password-token"
  SLACK_BOT_TOKEN: "xoxb-your-slack-token"
  API_KEY_1: "generate-a-secure-api-key"
  API_KEY_2: "generate-another-secure-api-key"
  DB_PASSWORD: "secure-database-password"
```

Generate secure API keys:

```bash
openssl rand -hex 32
```

### 2. TLS Certificates

Option A: Use cert-manager (recommended)

```bash
# Ingress is already configured for cert-manager
# Just ensure you have a ClusterIssuer named 'letsencrypt-prod'
```

Option B: Manual certificates

```bash
# Create TLS secret
kubectl create secret tls traverse-tls \
  --cert=server.crt \
  --key=server.key \
  --namespace=traverse
```

### 3. 1Password Credentials

Create the 1Password Connect credentials secret:

```bash
# You need the 1password-credentials.json file from 1Password CLI
kubectl create secret generic op-credentials \
  --from-file=1password-credentials.json=./1password-credentials.json \
  --namespace=traverse
```

## Usage

### Access the Service

From within cluster:

```bash
kubectl run --rm -it test --image=curlimages/curl --restart=Never -- \
  http://traverse.traverse.svc.cluster.local:8080/health
```

From outside (with ingress):

```bash
curl https://traverse.company.com/health
```

### Port Forward for Local Testing

```bash
# Forward Traverse API
kubectl port-forward -n traverse svc/traverse 8080:8080

# Forward 1Password Connect (if needed)
kubectl port-forward -n traverse svc/op-connect-api 8081:8080
```

### Request a Secret

```bash
curl -X POST http://localhost:8080/v1/secrets/request \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "secret_path": "prod/api-keys/stripe",
    "reason": "Production deployment",
    "duration": "1h"
  }'
```

## Monitoring

### Check Pod Status

```bash
kubectl get pods -n traverse
kubectl logs -n traverse deployment/traverse -f
```

### Prometheus Metrics

Traverse exposes metrics on port 9090:

```bash
kubectl port-forward -n traverse svc/traverse 9090:9090
curl http://localhost:9090/metrics
```

### Health Checks

```bash
kubectl port-forward -n traverse svc/traverse 8081:8081
curl http://localhost:8081/health
```

## Scaling

### Scale Traverse

```bash
kubectl scale deployment traverse --replicas=5 -n traverse
```

### Scale PostgreSQL

For production, consider using a managed PostgreSQL service or operators like:
- CloudNativePG
- Zalando Postgres Operator

## Security Hardening

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: traverse-network-policy
  namespace: traverse
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: traverse
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app.kubernetes.io/name: postgres
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - podSelector:
        matchLabels:
          app.kubernetes.io/name: op-connect-api
    ports:
    - protocol: TCP
      port: 8080
```

### Pod Security

The deployment already includes:
- `runAsNonRoot: true`
- `readOnlyRootFilesystem: true`
- Dropped capabilities
- Resource limits

## Troubleshooting

### Pod Not Starting

```bash
# Check events
kubectl get events -n traverse --sort-by='.lastTimestamp'

# Check logs
kubectl logs -n traverse deployment/traverse --previous
```

### Database Connection Issues

```bash
# Test database connectivity
kubectl run -it --rm debug --image=postgres:16-alpine --restart=Never -- \
  psql postgres://traverse:PASSWORD@postgres.traverse.svc.cluster.local:5432/traverse
```

### 1Password Connect Issues

```bash
# Check Connect API logs
kubectl logs -n traverse deployment/op-connect-api

# Test connectivity
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  http://op-connect-api.traverse.svc.cluster.local:8080/health
```

## Cleanup

```bash
kubectl delete namespace traverse
```
