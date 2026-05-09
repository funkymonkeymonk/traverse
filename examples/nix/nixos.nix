# NixOS Module Example

This example shows how to deploy Traverse using the NixOS module.

## Configuration

```nix
# /etc/nixos/traverse-configuration.nix
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
        host = "0.0.0.0";
        port = 8080;
        tls = {
          certFile = "/var/lib/traverse/certs/server.crt";
          keyFile = "/var/lib/traverse/certs/server.key";
        };
      };
      
      # Authentication
      auth = {
        type = "api_key";
        api_keys = [
          {
            key = "@API_KEY_1@";  # Will be substituted from environment
            client_id = "nixos-ci";
            allowed_paths = [ "prod/*" "shared/*" ];
            rate_limit = 30;
          }
        ];
      };
      
      # Secret providers
      providers = {
        default = "local";
        local = {
          type = "local";
          base_path = "/var/lib/traverse/secrets";
          encryption = {
            type = "age";
            recipient = "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p";
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
      
      # Policies
      policies = {
        default = {
          required_approvals = 1;
          max_duration = "1h";
          request_timeout = "5m";
          allow_self_approval = false;
        };
      };
      
      # Storage
      storage = {
        type = "postgresql";
        postgresql = {
          host = "localhost";
          port = 5432;
          database = "traverse";
          user = "traverse";
          password = "@DB_PASSWORD@";
          sslMode = "require";
        };
      };
      
      # Audit
      audit = {
        type = "file";
        file = {
          path = "/var/log/traverse/audit.log";
          max_size = 100;
          max_backups = 30;
        };
      };
      
      # Metrics
      metrics = {
        enabled = true;
        type = "prometheus";
        prometheus = {
          port = 9090;
          path = "/metrics";
        };
      };
    };
    
    # Environment file for secrets (not in Nix store)
    environmentFile = "/var/lib/traverse/environment";
  };
  
  # PostgreSQL database
  services.postgresql = {
    enable = true;
    ensureDatabases = [ "traverse" ];
    ensureUsers = [
      {
        name = "traverse";
        ensureDBOwnership = true;
      }
    ];
  };
  
  # Create directories and set permissions
  systemd.tmpfiles.rules = [
    "d /var/lib/traverse 0750 traverse traverse -"
    "d /var/lib/traverse/secrets 0750 traverse traverse -"
    "d /var/lib/traverse/certs 0750 traverse traverse -"
    "d /var/log/traverse 0750 traverse traverse -"
  ];
  
  # User for Traverse
  users.users.traverse = {
    isSystemUser = true;
    group = "traverse";
    home = "/var/lib/traverse";
    createHome = true;
  };
  
  users.groups.traverse = {};
  
  # Firewall
  networking.firewall.allowedTCPPorts = [ 8080 9090 ];
  
  # Nginx reverse proxy (optional)
  services.nginx = {
    enable = true;
    recommendedProxySettings = true;
    recommendedTlsSettings = true;
    
    virtualHosts."traverse.company.com" = {
      enableACME = true;
      forceSSL = true;
      
      locations."/" = {
        proxyPass = "http://127.0.0.1:8080";
        proxyWebsockets = true;
      };
    };
  };
  
  # Backup configuration
  services.postgresqlBackup = {
    enable = true;
    location = "/var/backup/postgresql";
    startAt = "*-*-* 02:00:00";
  };
}
```

## Secrets Management

Create the environment file outside of the Nix store:

```bash
sudo mkdir -p /var/lib/traverse
sudo tee /var/lib/traverse/environment > /dev/null << 'EOF'
API_KEY_1=your-secure-api-key-here
SLACK_BOT_TOKEN=xoxb-your-slack-token
DB_PASSWORD=your-database-password
EOF

sudo chmod 600 /var/lib/traverse/environment
sudo chown traverse:traverse /var/lib/traverse/environment
```

## Deployment

```bash
# Switch to new configuration
sudo nixos-rebuild switch -I nixos-config=/etc/nixos/traverse-configuration.nix

# Check service status
sudo systemctl status traverse

# View logs
sudo journalctl -u traverse -f
```

## Using with Flakes

```nix
# flake.nix
{
  description = "NixOS configuration with Traverse";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    traverse.url = "github:funkymonkeymonk/traverse";
  };

  outputs = { self, nixpkgs, traverse }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        traverse.nixosModules.default
        ./traverse-configuration.nix
      ];
    };
  };
}
```

Build and deploy:

```bash
sudo nixos-rebuild switch --flake .#myhost
```
