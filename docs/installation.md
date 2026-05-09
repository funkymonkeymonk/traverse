# Installation Guide

This guide shows you how to install Traverse on various platforms.

## Prerequisites

Before installing Traverse, ensure you have:

- **Go 1.23+** (for building from source)
- **Docker and Docker Compose** (for containerized deployment)
- **PostgreSQL 14+** or **SQLite** (for production or development respectively)
- **Access to a secret provider** (1Password, Vault, etc.)

## Docker Installation (Recommended)

### Quick Start

```bash
# Pull the latest image
docker pull funkymonkeymonk/traverse:latest

# Create a minimal configuration
mkdir -p ~/traverse/{config,secrets,data}
cat > ~/traverse/config/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080

auth:
  type: "api_key"
  api_keys:
    - key: "your_secure_api_key"
      client_id: "my-app"
      allowed_paths: ["*"]

providers:
  default: "local"
  local:
    type: "local"
    base_path: "/var/lib/traverse/secrets"

storage:
  type: "sqlite"
  sqlite:
    path: "/var/lib/traverse/traverse.db"

audit:
  type: "stdout"
EOF

# Run Traverse
docker run -d \
  --name traverse \
  -p 8080:8080 \
  -v ~/traverse/config/config.yaml:/etc/traverse/config.yaml:ro \
  -v ~/traverse/secrets:/var/lib/traverse/secrets \
  -v ~/traverse/data:/var/lib/traverse \
  funkymonkeymonk/traverse:latest

# Verify it's running
curl http://localhost:8080/health
```

### Docker Compose

For a complete production setup with PostgreSQL and monitoring:

```bash
# Copy the example compose file
cp examples/docker-compose.yml docker-compose.yml

# Create environment file
cat > .env << 'EOF'
DB_PASSWORD=your_secure_db_password
OP_CONNECT_TOKEN=your_1password_token
DUO_INTEGRATION_KEY=your_duo_key
DUO_SECRET_KEY=your_duo_secret
SLACK_BOT_TOKEN=xoxb-your-slack-token
SENTINEL_API_KEY_1=your_api_key_1
EOF

# Start all services
docker-compose up -d

# Check status
docker-compose ps
```

## Binary Installation

### Download Pre-built Binary

```bash
# Detect your OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

# Download latest release
curl -L "https://github.com/funkymonkeymonk/traverse/releases/latest/download/traverse-${OS}-${ARCH}" \
  -o /usr/local/bin/traverse

# Make executable
chmod +x /usr/local/bin/traverse

# Verify installation
traverse --version
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/funkymonkeymonk/traverse.git
cd traverse

# Build the binary
go build -o traverse ./cmd/traverse

# Install to system (optional)
sudo cp traverse /usr/local/bin/

# Or run directly
./traverse --config config.yaml
```

## Package Manager Installation

### Homebrew (macOS/Linux)

```bash
# Add tap (when available)
brew tap funkymonkeymonk/tap

# Install Traverse
brew install traverse

# Start service
brew services start traverse
```

### Nix/NixOS

#### Using Flakes

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    traverse.url = "github:funkymonkeymonk/traverse";
  };

  outputs = { self, nixpkgs, traverse }: {
    packages.default = traverse.packages.default;
  };
}
```

#### NixOS Module

```nix
# configuration.nix
{ config, pkgs, ... }:
{
  imports = [ 
    (fetchTarball "https://github.com/funkymonkeymonk/traverse/archive/main.tar.gz")
  ];

  services.traverse = {
    enable = true;
    settings = {
      server.port = 8080;
      # ... configuration
    };
  };
}
```

#### nix-darwin (macOS)

```nix
# flake.nix
{
  inputs = {
    nix-darwin.url = "github:LnL7/nix-darwin";
    traverse.url = "github:funkymonkeymonk/traverse";
  };

  outputs = { self, nix-darwin, traverse }: {
    darwinConfigurations.myhost = nix-darwin.lib.darwinSystem {
      modules = [
        traverse.darwinModules.default
        {
          services.traverse = {
            enable = true;
            settings.server.port = 8080;
          };
        }
      ];
    };
  };
}
```

## Systemd Service (Linux)

Create a systemd service for automatic startup:

```bash
# Create user and directories
sudo useradd -r -s /bin/false traverse
sudo mkdir -p /etc/traverse /var/lib/traverse
sudo chown traverse:traverse /var/lib/traverse

# Create systemd service file
sudo tee /etc/systemd/system/traverse.service > /dev/null << 'EOF'
[Unit]
Description=Traverse MFA Secrets Proxy
After=network.target

[Service]
Type=simple
User=traverse
Group=traverse
ExecStart=/usr/local/bin/traverse --config /etc/traverse/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=traverse

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/traverse
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable traverse
sudo systemctl start traverse

# Check status
sudo systemctl status traverse
sudo journalctl -u traverse -f
```

## macOS Service

### Using launchd

```bash
# Create LaunchAgent plist
cat > ~/Library/LaunchAgents/com.funkymonkeymonk.traverse.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.funkymonkeymonk.traverse</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/traverse</string>
        <string>--config</string>
        <string>/etc/traverse/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/traverse/traverse.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/traverse/traverse.error.log</string>
</dict>
</plist>
EOF

# Load the service
launchctl load ~/Library/LaunchAgents/com.funkymonkeymonk.traverse.plist

# Start the service
launchctl start com.funkymonkeymonk.traverse

# Check status
launchctl list | grep traverse
```

## Windows Service

### Using NSSM (Non-Sucking Service Manager)

```powershell
# Download NSSM
Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "nssm.zip"
Expand-Archive -Path "nssm.zip" -DestinationPath "C:\nssm"

# Install Traverse as a service
C:\nssm\nssm.exe install Traverse "C:\Program Files\Traverse\traverse.exe"
C:\nssm\nssm.exe set Traverse AppParameters "--config C:\ProgramData\Traverse\config.yaml"
C:\nssm\nssm.exe set Traverse AppDirectory "C:\ProgramData\Traverse"

# Start the service
Start-Service Traverse

# Check status
Get-Service Traverse
```

## Kubernetes Installation

See the [Kubernetes examples](../examples/kubernetes/) for complete manifests.

Quick deployment:

```bash
# Apply manifests
kubectl apply -f examples/kubernetes/namespace.yaml
kubectl apply -f examples/kubernetes/configmap.yaml
kubectl apply -f examples/kubernetes/secret.yaml
kubectl apply -f examples/kubernetes/deployment.yaml
kubectl apply -f examples/kubernetes/service.yaml

# Check deployment
kubectl get pods -n traverse
kubectl logs -n traverse deployment/traverse
```

## Post-Installation

### Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","version":"1.0.0","timestamp":"2024-01-15T10:00:00Z"}
```

### Create Your First Secret

```bash
# Create a test secret (using local provider)
echo '{"api_key": "test_key_123"}' | \
  traverse-cli secrets create dev/test-api-key --values -

# Request access
curl -X POST http://localhost:8080/v1/secrets/request \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "secret_path": "dev/test-api-key",
    "reason": "Testing installation",
    "duration": "15m"
  }'
```

### Configure Your Client

Create a client configuration file:

```yaml
# ~/.traverse/config.yaml
server_url: "http://localhost:8080"
api_key: "your_api_key"
default_duration: "1h"
```

## Next Steps

- **[Configuration Guide](configuration.md)** - Learn how to configure Traverse
- **[Provider Setup](providers.md)** - Configure 1Password, Vault, and other providers
- **[Notification Setup](notifications.md)** - Set up Duo, Slack, and other notification channels
- **[Security Best Practices](../README.md#security-best-practices)** - Secure your installation

## Troubleshooting Installation

### Port Already in Use

```bash
# Check what's using port 8080
sudo lsof -i :8080

# Kill the process or change Traverse port
# Edit config.yaml:
server:
  port: 8081
```

### Permission Denied

```bash
# Fix permissions on data directory
sudo chown -R $(whoami):$(whoami) /var/lib/traverse

# Or use a different data directory
server:
  data_dir: "~/traverse/data"
```

### Configuration Not Found

Ensure your configuration file exists and is readable:

```bash
# Check file exists
ls -la /etc/traverse/config.yaml

# Validate YAML syntax
cat /etc/traverse/config.yaml | yq

# Check Traverse can read it
traverse --config /etc/traverse/config.yaml --validate
```
