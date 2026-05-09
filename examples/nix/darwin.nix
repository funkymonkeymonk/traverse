# nix-darwin Module Example

This example shows how to deploy Traverse on macOS using nix-darwin.

## Configuration

```nix
# ~/darwin-configuration.nix
{ config, pkgs, lib, ... }:

{
  # Import the Traverse module
  imports = [
    (fetchTarball "https://github.com/funkymonkeymonk/traverse/archive/main.tar.gz")
  ];

  # Enable Traverse service
  services.traverse = {
    enable = true;
    
    # Server configuration
    settings = {
      server = {
        host = "127.0.0.1";  # Local only on macOS
        port = 8080;
      };
      
      # Authentication
      auth = {
        type = "api_key";
        api_keys = [
          {
            key = "@API_KEY_1@";
            client_id = "macos-ci";
            allowed_paths = [ "dev/*" "shared/*" ];
            rate_limit = 30;
          }
        ];
      };
      
      # Use local file provider for macOS
      providers = {
        default = "local";
        local = {
          type = "local";
          base_path = "/var/lib/traverse/secrets";
          encryption = {
            type = "age";
            recipient = "@AGE_RECIPIENT@";
          };
        };
      };
      
      # Notifications
      notifications = {
        default = [ "slack" ];
        slack = {
          bot_token = "@SLACK_BOT_TOKEN@";
          approver_channel = "#security-approvals";
          dm_users = true;
        };
      };
      
      # SQLite for macOS
      storage = {
        type = "sqlite";
        sqlite = {
          path = "/var/lib/traverse/traverse.db";
        };
      };
      
      # Audit to file
      audit = {
        type = "file";
        file = {
          path = "/var/log/traverse/audit.log";
          max_size = 100;
          max_backups = 10;
        };
      };
    };
    
    # Environment file
    environmentFile = "/var/lib/traverse/environment";
  };
  
  # Create launchd service (managed by nix-darwin)
  launchd.daemons.traverse = {
    serviceConfig = {
      WorkingDirectory = "/var/lib/traverse";
      StandardOutPath = "/var/log/traverse/traverse.log";
      StandardErrorPath = "/var/log/traverse/traverse.error.log";
    };
  };
  
  # Create directories
  system.activationScripts.traverse.text = ''
    mkdir -p /var/lib/traverse/secrets
    mkdir -p /var/lib/traverse/certs
    mkdir -p /var/log/traverse
    chown -R root:wheel /var/lib/traverse
    chmod 755 /var/lib/traverse
  '';
}
```

## Using with Flakes

```nix
# flake.nix
{
  description = "Darwin configuration with Traverse";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    darwin.url = "github:LnL7/nix-darwin";
    darwin.inputs.nixpkgs.follows = "nixpkgs";
    traverse.url = "github:funkymonkeymonk/traverse";
  };

  outputs = { self, nixpkgs, darwin, traverse }: {
    darwinConfigurations.my-macbook = darwin.lib.darwinSystem {
      system = "aarch64-darwin";  # or "x86_64-darwin"
      modules = [
        traverse.darwinModules.default
        ./darwin-configuration.nix
      ];
    };
  };
}
```

## Secrets Management

Create the environment file:

```bash
sudo mkdir -p /var/lib/traverse
sudo tee /var/lib/traverse/environment > /dev/null << 'EOF'
API_KEY_1=your-secure-api-key-here
SLACK_BOT_TOKEN=xoxb-your-slack-token
AGE_RECIPIENT=age1ql3z7hjy54pw3...
EOF

sudo chmod 600 /var/lib/traverse/environment
```

## Deployment

```bash
# Build and switch
darwin-rebuild switch --flake .#my-macbook

# Check service status
sudo launchctl list | grep traverse

# View logs
sudo tail -f /var/log/traverse/traverse.log
```

## Homebrew Service Alternative

If not using nix-darwin, you can use Homebrew services:

```nix
# homebrew-traverse.nix
{ pkgs, ... }:

let
  traverse = pkgs.buildGoModule rec {
    pname = "traverse";
    version = "1.0.0";
    
    src = pkgs.fetchFromGitHub {
      owner = "funkymonkeymonk";
      repo = "traverse";
      rev = "v${version}";
      sha256 = "...";
    };
    
    vendorSha256 = "...";
  };
in
{
  homebrew.services.traverse = {
    enable = true;
    config = {
      ProgramArguments = [
        "${traverse}/bin/traverse"
        "--config"
        "/etc/traverse/config.yaml"
      ];
      RunAtLoad = true;
      KeepAlive = true;
      StandardOutPath = "/var/log/traverse/traverse.log";
      StandardErrorPath = "/var/log/traverse/traverse.error.log";
    };
  };
}
```
