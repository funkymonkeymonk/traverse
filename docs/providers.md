# Provider Setup Guides

This guide covers setting up secret providers for Traverse.

## Supported Providers

- [1Password Connect](#1password-connect)
- [HashiCorp Vault](#hashicorp-vault)
- [AWS Secrets Manager](#aws-secrets-manager)
- [Local File Provider](#local-file-provider)

---

## 1Password Connect

1Password Connect allows Traverse to access secrets stored in your 1Password vaults.

### Prerequisites

- 1Password Business or Enterprise account
- Access to create Connect servers and tokens

### Setup Steps

#### 1. Create a 1Password Connect Server

```bash
# Install 1Password CLI
brew install 1password-cli

# Sign in
op signin

# Create credentials file for Connect server
op connect server create "Traverse Connect" > 1password-credentials.json
```

#### 2. Create Access Token

```bash
# Create a token with appropriate vault access
op connect token create "Traverse Token" \
  --server "Traverse Connect" \
  --vault "Production Secrets"
```

Save the token securely - you'll need it for Traverse configuration.

#### 3. Configure Traverse

```yaml
providers:
  default: "1password"
  
  1password:
    type: "1password-connect"
    host: "https://op-connect.internal:8080"
    token: "${OP_CONNECT_TOKEN}"
    timeout: "10s"
    retry:
      max_retries: 3
      backoff: "exponential"
```

#### 4. Docker Compose Setup

```yaml
version: '3.8'
services:
  op-connect-api:
    image: 1password/connect-api:latest
    volumes:
      - ./1password-credentials.json:/home/opuser/.op/1password-credentials.json:ro
      - op-data:/home/opuser/.op/data
    ports:
      - "8081:8080"
    restart: unless-stopped

  op-connect-sync:
    image: 1password/connect-sync:latest
    volumes:
      - ./1password-credentials.json:/home/opuser/.op/1password-credentials.json:ro
      - op-data:/home/opuser/.op/data
    restart: unless-stopped

  traverse:
    image: funkymonkeymonk/traverse:latest
    environment:
      - OP_CONNECT_TOKEN=${OP_CONNECT_TOKEN}
    depends_on:
      - op-connect-api
    # ... rest of config

volumes:
  op-data:
```

### Path Mapping

1Password secrets are accessed using this path format:

```
op://<vault-name>/<item-name>/<field-name>
```

Example:
```yaml
# Request via Traverse API
{
  "secret_path": "op://Production/API Keys/stripe-api-key"
}
```

### Security Considerations

1. **Token Scope**: Limit tokens to specific vaults, use "view items" permission only
2. **Network**: Run Connect server on internal network only
3. **Rotation**: Rotate Connect tokens every 90 days
4. **Monitoring**: Monitor Connect server logs for unusual access patterns

### Troubleshooting

**Token Expired**
```bash
# Generate new token
op connect token create "Traverse Token New" \
  --server "Traverse Connect" \
  --vault "Production Secrets"

# Update Traverse config and restart
```

**Vault Not Found**
```bash
# List available vaults
op vault list

# Check token vault access
op connect token list --server "Traverse Connect"
```

---

## HashiCorp Vault

Vault integration supports multiple authentication methods and KV engine versions.

### Prerequisites

- Running Vault cluster (v1.10+)
- Appropriate policies configured

### Setup Steps

#### 1. Create Vault Policy

```hcl
# traverse-policy.hcl
path "secret/data/*" {
  capabilities = ["read"]
}

path "secret/metadata/*" {
  capabilities = ["list"]
}
```

```bash
# Write policy
vault policy write traverse-policy traverse-policy.hcl
```

#### 2. Configure Authentication

**AppRole (Recommended for production)**

```bash
# Enable AppRole auth
vault auth enable approle

# Create AppRole
vault write auth/approle/role/traverse \
  policies="traverse-policy" \
  token_ttl=1h \
  token_max_ttl=4h \
  secret_id_ttl=0 \
  token_num_uses=0

# Get Role ID
vault read auth/approle/role/traverse/role-id
# Key        Value
# ---        -----
# role_id    5e4e55e6-1d52-7c9d-e89b-197b73e66b15

# Generate Secret ID
vault write -f auth/approle/role/traverse/secret-id
# Key                   Value
# ---                   -----
# secret_id             7263f698-6792-a635-6072-cb5e4f45c842
# secret_id_accessor    564a6458-0774-8c1e-623e-033a44592b87
```

**Token Auth (Development only)**

```bash
# Create a token
vault token create -policy=traverse-policy -ttl=768h
# Key                  Value
# ---                  -----
# token                hvs.CAESIIoJQL3...
```

#### 3. Configure Traverse

**AppRole Auth**

```yaml
providers:
  vault:
    type: "hashicorp-vault"
    address: "https://vault.internal:8200"
    auth:
      type: "approle"
      role_id: "${VAULT_ROLE_ID}"
      secret_id: "${VAULT_SECRET_ID}"
    tls:
      ca_cert: "/etc/traverse/certs/vault-ca.crt"
    kv_version: "v2"
```

**Token Auth**

```yaml
providers:
  vault:
    type: "hashicorp-vault"
    address: "https://vault.internal:8200"
    auth:
      type: "token"
      token: "${VAULT_TOKEN}"
    kv_version: "v2"
```

**Kubernetes Auth**

```yaml
providers:
  vault:
    type: "hashicorp-vault"
    address: "https://vault.internal:8200"
    auth:
      type: "kubernetes"
      kubernetes:
        role: "traverse"
        service_account_path: "/var/run/secrets/kubernetes.io/serviceaccount/token"
    kv_version: "v2"
```

#### 4. Kubernetes Setup

Create a service account and configure Vault:

```bash
# Enable Kubernetes auth
vault auth enable kubernetes

# Configure Kubernetes auth
vault write auth/kubernetes/config \
  token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443" \
  kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt

# Create role
vault write auth/kubernetes/role/traverse \
  bound_service_account_names=traverse \
  bound_service_account_namespaces=default \
  policies=traverse-policy \
  ttl=1h
```

```yaml
# kubernetes/service-account.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: traverse
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: traverse-tokenreview-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: traverse
  namespace: default
```

### Path Mapping

Vault KV v2 paths in Traverse:

```
# Request format
vault://<path>

# Examples
vault://secret/data/api-keys/stripe
vault://secret/data/database/production/password
```

### KV Engine Versions

**KV v1** (no versioning)
```yaml
providers:
  vault:
    kv_version: "v1"
```
Path: `vault://secret/api-keys/stripe`

**KV v2** (with versioning)
```yaml
providers:
  vault:
    kv_version: "v2"
```
Path: `vault://secret/data/api-keys/stripe`

### Troubleshooting

**Permission Denied**
```bash
# Check token capabilities
vault token capabilities secret/data/api-keys/stripe

# Review policy
vault policy read traverse-policy
```

**Token Expired**
```bash
# Renew token
vault token renew -increment=4h <token>

# Or generate new AppRole secret
vault write -f auth/approle/role/traverse/secret-id
```

---

## AWS Secrets Manager

AWS Secrets Manager integration uses the AWS SDK default credential chain.

### Prerequisites

- AWS account with Secrets Manager access
- IAM role or user with appropriate permissions

### Setup Steps

#### 1. Create IAM Policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "arn:aws:secretsmanager:*:*:secret:traverse/*"
    },
    {
      "Effect": "Allow",
      "Action": "secretsmanager:ListSecrets",
      "Resource": "*"
    }
  ]
}
```

#### 2. Configure Credentials

**IAM Role (EC2/EKS)**

```bash
# Attach role to EC2 instance or EKS service account
# No additional configuration needed
```

**Environment Variables**

```bash
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1
```

**Shared Credentials File**

```ini
# ~/.aws/credentials
[traverse]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

# ~/.aws/config
[profile traverse]
region = us-east-1
```

#### 3. Configure Traverse

```yaml
providers:
  aws:
    type: "aws-secrets-manager"
    region: "us-east-1"
    # Auth via IAM role or environment (recommended)
    # auth:
    #   access_key_id: "${AWS_ACCESS_KEY_ID}"
    #   secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
```

### Path Mapping

```
# Request format
aws://<secret-name>

# Examples
aws://prod/api-keys/stripe
aws://database/production/password
```

### Cross-Account Access

```yaml
providers:
  aws:
    type: "aws-secrets-manager"
    region: "us-east-1"
    role_arn: "arn:aws:iam::123456789:role/traverse-secrets-role"
    external_id: "traverse-external-id"  # Optional but recommended
```

### Troubleshooting

**Access Denied**
```bash
# Test AWS access
aws secretsmanager get-secret-value --secret-id prod/api-keys/stripe

# Check IAM permissions
aws iam simulate-principal-policy \
  --policy-source-arn arn:aws:iam::123456789:user/traverse \
  --action-names secretsmanager:GetSecretValue \
  --resource-arns arn:aws:secretsmanager:us-east-1:123456789:secret:prod/api-keys/stripe
```

---

## Local File Provider

The local file provider stores secrets as encrypted files on disk.

### Prerequisites

- Age or GPG for encryption
- Secure directory for storage

### Setup Steps

#### 1. Install Age

```bash
# macOS
brew install age

# Linux
apt-get install age  # Debian/Ubuntu
yum install age      # RHEL/CentOS

# Or download binary
curl -L https://github.com/FiloSottile/age/releases/download/v1.1.1/age-v1.1.1-linux-amd64.tar.gz | tar xz
sudo mv age/age* /usr/local/bin/
```

#### 2. Generate Age Key

```bash
# Generate new key pair
age-keygen -o /etc/traverse/keys/age.key

# Extract public key
cat /etc/traverse/keys/age.key | grep "public key" | awk '{print $3}'
# age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

#### 3. Configure Traverse

```yaml
providers:
  default: "local"
  
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"
    encryption:
      type: "age"
      recipient: "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"
```

#### 4. Create Secrets

```bash
# Create secret file
echo '{"api_key": "sk_live_123", "api_secret": "secret_456"}' | \
  age -r age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p \
  -o /var/lib/traverse/secrets/prod/api-keys/stripe.json.age

# Verify
age -d -i /etc/traverse/keys/age.key /var/lib/traverse/secrets/prod/api-keys/stripe.json.age
```

### Path Mapping

```
# Request format
local://<path>

# Examples (maps to files)
local://prod/api-keys/stripe     # /var/lib/traverse/secrets/prod/api-keys/stripe.json.age
local://database/password        # /var/lib/traverse/secrets/database/password.json.age
```

### GPG Alternative

```yaml
providers:
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"
    encryption:
      type: "pgp"
      public_key: "/etc/traverse/keys/public.asc"
      private_key: "/etc/traverse/keys/private.asc"
```

```bash
# Generate GPG key
gpg --full-generate-key

# Export keys
gpg --export --armor user@example.com > /etc/traverse/keys/public.asc
gpg --export-secret-keys --armor user@example.com > /etc/traverse/keys/private.asc

# Encrypt secret
echo '{"key": "value"}' | gpg --encrypt --armor -r user@example.com \
  -o /var/lib/traverse/secrets/secret.json.gpg
```

### Security Best Practices

1. **Key Management**: Store private keys in HSM or hardware token
2. **File Permissions**: Restrict access to secret files
   ```bash
   chmod 700 /var/lib/traverse/secrets
   chmod 600 /etc/traverse/keys/age.key
   ```
3. **Backup**: Backup encrypted files, keep keys separate
4. **Rotation**: Regularly rotate encryption keys

---

## Provider Selection

Configure multiple providers and select per-request:

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

Override per-request:

```bash
# Use specific provider
curl -X POST http://localhost:8080/v1/secrets/request \
  -H "Authorization: Bearer API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "secret_path": "vault://secret/data/api-keys/stripe",
    "reason": "Need Vault secret"
  }'
```

Or use provider prefixes:

```
op://vault-name/item/field      # 1Password
vault://path/to/secret          # Vault
aws://secret-name               # AWS Secrets Manager
local://path/to/file            # Local files
```
