# Shared options schema for Traverse
# Exported from lib/traverse-options.nix for type checking and reuse
{lib}:
with lib; {
  # Main enable switch
  enable = mkEnableOption "Traverse - MFA secrets proxy with approval workflows";

  # Package to use
  package = mkOption {
    type = types.package;
    default = null;
    description = "The Traverse package to use.";
  };

  # Service port
  port = mkOption {
    type = types.port;
    default = 8080;
    description = "Port on which Traverse will listen.";
  };

  # Service host
  host = mkOption {
    type = types.str;
    default = "127.0.0.1";
    description = "Host address on which Traverse will bind.";
  };

  # Service settings (YAML config)
  settings = mkOption {
    type = types.attrsOf types.anything;
    default = {};
    description = ''
      Traverse configuration settings. These will be serialized to YAML
      and placed in the configuration file.

      See the Traverse documentation for available options:
      - server: { host, port, tls }
      - auth: { type, api_keys }
      - providers: secret provider configuration
      - notifications: notification provider configuration
      - policies: approval policies
      - audit: audit logging configuration
      - storage: database configuration
    '';
    example = literalExpression ''
      {
        server = {
          host = "0.0.0.0";
          port = 8080;
        };
        auth = {
          type = "api_key";
        };
        storage = {
          type = "sqlite";
          sqlite.path = "/var/lib/traverse/traverse.db";
        };
      }
    '';
  };

  # Environment files for secrets (agenix/ragenix/sops-nix integration)
  environmentFiles = mkOption {
    type = types.listOf types.path;
    default = [];
    description = ''
      List of environment files to load. These are typically provided by
      agenix, ragenix, or sops-nix for secret management.

      Each file should contain KEY=value pairs that will be sourced
      before starting the Traverse service.
    '';
  };

  # Data directory
  dataDir = mkOption {
    type = types.str;
    default = "/var/lib/traverse";
    description = "Directory where Traverse stores its data (database, logs).";
  };

  # User and group
  user = mkOption {
    type = types.str;
    default = "traverse";
    description = "User account under which Traverse runs.";
  };

  group = mkOption {
    type = types.str;
    default = "traverse";
    description = "Group under which Traverse runs.";
  };

  # Log level
  logLevel = mkOption {
    type = types.enum ["debug" "info" "warn" "error"];
    default = "info";
    description = "Log level for Traverse.";
  };

  # Open firewall
  openFirewall = mkOption {
    type = types.bool;
    default = false;
    description = ''
      Whether to open the firewall for Traverse.

      Note: Only applies to NixOS. On Darwin, firewall must be
      configured separately.
    '';
  };

  # Health check configuration
  healthCheck = {
    enable = mkOption {
      type = types.bool;
      default = true;
      description = "Enable health check endpoint.";
    };

    path = mkOption {
      type = types.str;
      default = "/health";
      description = "Path for health check endpoint.";
    };

    interval = mkOption {
      type = types.int;
      default = 30;
      description = "Health check interval in seconds (for systemd/launchd).";
    };
  };

  # Log rotation
  logRotation = {
    enable = mkOption {
      type = types.bool;
      default = true;
      description = "Enable log rotation.";
    };

    maxSize = mkOption {
      type = types.str;
      default = "100M";
      description = "Maximum log file size before rotation.";
    };

    maxFiles = mkOption {
      type = types.int;
      default = 10;
      description = "Maximum number of rotated log files to keep.";
    };
  };
}
