# Traverse

[![Go Report Card](https://goreportcard.com/badge/github.com/funkymonkeymonk/traverse)](https://goreportcard.com/report/github.com/funkymonkeymonk/traverse)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **MFA-Protected Secrets Proxy with Approval Workflows**

Traverse is a secure proxy service that adds multi-factor authentication (MFA) and approval workflows to your existing secret management systems like 1Password, HashiCorp Vault, and more.

## Quick Start

Get Traverse running in 5 minutes:

```bash
# Clone the repository
git clone https://github.com/funkymonkeymonk/traverse.git
cd traverse

# Start with Docker Compose
docker-compose -f examples/docker-compose.yaml up -d

# Configure your client
cat > ~/.traverse/config.yaml << 'EOF'
server_url: "http://localhost:8080"
api_key: "dev_api_key_change_in_production"
EOF

# Request a secret
curl -X POST http://localhost:8080/v1/secrets/request \
  -H "Authorization: Bearer dev_api_key_change_in_production" \
  -H "Content-Type: application/json" \
  -d '{
    "secret_path": "dev/api-keys/stripe",
    "reason": "Testing Traverse deployment",
    "duration": "1h"
  }'
```

## What is Traverse?

Traverse sits between your applications and your secret providers, adding a layer of security through:

- **Approval Workflows**: Require human approval before secrets are accessed
- **MFA Integration**: Duo Push, Slack interactive messages, and more
- **Audit Logging**: Complete audit trail of all secret access
- **Time-Bound Access**: Secrets are only accessible for approved time windows
- **Multiple Providers**: Works with 1Password, Vault, AWS Secrets Manager, and local files

## Features

| Feature | Description |
|---------|-------------|
| **Secret Providers** | 1Password Connect, HashiCorp Vault, AWS Secrets Manager, Local encrypted files |
| **MFA Methods** | Duo Push, Slack interactive messages, PagerDuty, Telegram, Email |
| **Authentication** | API keys, mTLS (mutual TLS) |
| **Policies** | Path-based approval rules, dual authorization for sensitive paths |
| **Audit** | Multiple backends: file, syslog, webhook, database |
| **Storage** | SQLite (dev), PostgreSQL (production) |
| **Metrics** | Prometheus metrics endpoint |

## Installation

### Docker (Recommended)

```bash
docker run -d \
  --name traverse \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/traverse/config.yaml:ro \
  -v $(pwd)/secrets:/var/lib/traverse/secrets \
  funkymonkeymonk/traverse:latest
```

### Binary

Download the latest release:

```bash
curl -L https://github.com/funkymonkeymonk/traverse/releases/latest/download/traverse-$(uname -s)-$(uname -m) \
  -o /usr/local/bin/traverse
chmod +x /usr/local/bin/traverse
```

### Nix/NixOS

```nix
# flake.nix
{
  inputs.traverse.url = "github:funkymonkeymonk/traverse";
  
  outputs = { self, nixpkgs, traverse }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      modules = [
        traverse.nixosModules.default
        {
          services.traverse = {
            enable = true;
            settings = {
              server.port = 8080;
              # ... your config
            };
          };
        }
      ];
    };
  };
}
```

## Configuration

Create a minimal configuration file:

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8080

auth:
  type: "api_key"
  api_keys:
    - key: "your_api_key_here"
      client_id: "my-app"
      allowed_paths: ["*"]

providers:
  default: "local"
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"

notifications:
  default: ["slack"]
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"

policies:
  default:
    required_approvals: 1
    max_duration: "1h"
    request_timeout: "5m"

storage:
  type: "sqlite"
  sqlite:
    path: "/var/lib/traverse/traverse.db"
```

See [docs/configuration.md](docs/configuration.md) for complete configuration reference.

## Documentation

- **[Installation Guide](docs/installation.md)** - Detailed installation instructions
- **[Configuration Reference](docs/configuration.md)** - Complete configuration options
- **[Provider Setup](docs/providers.md)** - Configure 1Password, Vault, and other providers
- **[Notification Setup](docs/notifications.md)** - Set up Duo, Slack, and other notification channels
- **[Architecture](docs/architecture.md)** - Technical overview and design decisions
- **[API Documentation](docs/api.md)** - REST API reference

## Usage Example

### 1. Request a Secret

```bash
curl -X POST http://localhost:8080/v1/secrets/request \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "secret_path": "prod/database/password",
    "reason": "Database migration for v2.0",
    "duration": "30m"
  }'
```

Response:
```json
{
  "request_id": "req_abc123",
  "status": "pending_approval",
  "message": "Request sent to approvers via Slack",
  "expires_at": "2024-01-15T10:30:00Z"
}
```

### 2. Approver Receives Notification

Your approver receives a Slack message:

> **Secret Access Request**
> 
> Client: `deployment-bot`  
> Path: `prod/database/password`  
> Reason: Database migration for v2.0  
> Duration: 30 minutes
> 
> [✅ Approve] [❌ Deny]

### 3. Access the Secret

Once approved:

```bash
curl -X GET http://localhost:8080/v1/secrets/prod/database/password \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Response:
```json
{
  "path": "prod/database/password",
  "values": {
    "password": "super_secret_password_123"
  },
  "expires_at": "2024-01-15T10:30:00Z"
}
```

## Deployment Examples

- **[Docker Compose](examples/docker-compose.yml)** - Full stack with PostgreSQL, 1Password Connect, and monitoring
- **[Kubernetes](examples/kubernetes/)** - K8s manifests with external secrets
- **[NixOS](examples/nix/)** - NixOS module configuration

## Security Best Practices

1. **Use mTLS in production** - Client certificates provide stronger authentication than API keys
2. **Enable audit logging** - Send logs to a SIEM for security monitoring
3. **Set short expiration times** - Reduce the window of exposure
4. **Require dual approval** - For sensitive paths, require two approvers
5. **Monitor metrics** - Watch for unusual access patterns
6. **Regular key rotation** - Rotate API keys and provider credentials

See [docs/security.md](docs/security.md) for detailed security guidelines.

## Troubleshooting

### Common Issues

**Connection refused**
```bash
# Check if Traverse is running
curl http://localhost:8080/health

# Check logs
docker logs traverse
```

**Authentication failed**
- Verify your API key is correct
- Check that the client has access to the requested path
- Ensure the key hasn't expired

**Approval not received**
- Verify notification provider configuration
- Check that approvers are configured correctly
- Review notification provider logs

See [docs/troubleshooting.md](docs/troubleshooting.md) for more.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- 📖 [Documentation](https://github.com/funkymonkeymonk/traverse/tree/main/docs)
- 🐛 [Issue Tracker](https://github.com/funkymonkeymonk/traverse/issues)
- 💬 [Discussions](https://github.com/funkymonkeymonk/traverse/discussions)
