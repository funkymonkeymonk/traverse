# Nix Examples

Examples for deploying Traverse using Nix.

## Available Examples

- **nixos.nix** - NixOS module configuration
- **darwin.nix** - nix-darwin module for macOS
- **home-manager.nix** - Home Manager configuration (client tools)

## Quick Start

### NixOS

```bash
# Copy the example
sudo cp nixos.nix /etc/nixos/traverse.nix

# Edit configuration
sudo nano /etc/nixos/traverse.nix

# Include in main configuration
echo 'imports = [ ./traverse.nix ];' | sudo tee -a /etc/nixos/configuration.nix

# Deploy
sudo nixos-rebuild switch
```

### macOS with nix-darwin

```bash
# Copy the example
cp darwin.nix ~/.config/nix-darwin/traverse.nix

# Edit configuration
nano ~/.config/nix-darwin/traverse.nix

# Include in flake.nix or main configuration

# Deploy
darwin-rebuild switch
```

## Flake Template

Use this template for a complete Nix-based deployment:

```nix
{
  description = "Traverse deployment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    traverse.url = "github:funkymonkeymonk/traverse";
  };

  outputs = { self, nixpkgs, traverse }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      nixosConfigurations.traverse-server = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          traverse.nixosModules.default
          {
            services.traverse = {
              enable = true;
              settings = {
                server.port = 8080;
                # ... your configuration
              };
            };
          }
        ];
      };
      
      # Development shell
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          traverse.packages.${system}.default
          postgresql
          age
        ];
      };
    };
}
```

## Secrets Management

All examples use environment files for secrets, keeping them out of the Nix store:

```bash
# Create environment file
sudo mkdir -p /var/lib/traverse
sudo tee /var/lib/traverse/environment << 'EOF'
API_KEY_1=$(openssl rand -hex 32)
SLACK_BOT_TOKEN=xoxb-your-token
DB_PASSWORD=$(openssl rand -hex 16)
EOF

sudo chmod 600 /var/lib/traverse/environment
```

## Testing

```bash
# Enter development shell
nix develop

# Test configuration
traverse --config /etc/traverse/config.yaml --validate

# Start manually for testing
traverse --config /etc/traverse/config.yaml
```
