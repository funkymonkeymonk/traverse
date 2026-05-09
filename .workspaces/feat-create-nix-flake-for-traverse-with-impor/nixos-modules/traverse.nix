# NixOS module for Traverse
# Provides systemd service configuration for Traverse on NixOS
{
  config,
  lib,
  pkgs,
  ...
}:
with lib; let
  cfg = config.services.traverse;

  # Import shared options
  traverseOptions = import ../lib/traverse-options.nix {inherit lib;};

  # Default package if not specified
  defaultPackage = import ../packages/traverse.nix {inherit pkgs;};

  # Generate configuration file from settings
  configFile = pkgs.writeText "traverse.yaml" (
    builtins.toJSON ({
        server = {
          host = cfg.host;
          port = cfg.port;
        };
        log_level = cfg.logLevel;
      }
      // cfg.settings)
  );

  # Build environment file loading script
  envFileScript =
    concatMapStrings (file: ''
      if [ -f "${file}" ]; then
        source "${file}"
      fi
    '')
    cfg.environmentFiles;
in {
  options.services.traverse = traverseOptions;

  config = mkIf cfg.enable {
    # Create traverse user and group
    users.users.traverse = {
      description = "Traverse service user";
      isSystemUser = true;
      group = cfg.group;
      home = cfg.dataDir;
      createHome = true;
    };

    users.groups.traverse = {};

    # Create data directory
    systemd.tmpfiles.rules = [
      "d '${cfg.dataDir}' 0750 ${cfg.user} ${cfg.group} - -"
      "d '${cfg.dataDir}/logs' 0750 ${cfg.user} ${cfg.group} - -"
    ];

    # Systemd service
    systemd.services.traverse = {
      description = "Traverse - MFA secrets proxy with approval workflows";
      after = ["network.target"];
      wantedBy = ["multi-user.target"];

      serviceConfig = {
        Type = "simple";
        User = cfg.user;
        Group = cfg.group;

        ExecStartPre = ''
          ${pkgs.bash}/bin/bash -c '${envFileScript}'
        '';

        ExecStart = ''
          ${cfg.package or defaultPackage}/bin/traverse server \
            --config ${configFile}
        '';

        Restart = "on-failure";
        RestartSec = 5;

        # Security hardening
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [cfg.dataDir];

        # Resource limits
        LimitNOFILE = 65536;

        # Logging
        StandardOutput = "journal";
        StandardError = "journal";
        SyslogIdentifier = "traverse";
      };

      # Health check
      preStart = mkIf cfg.healthCheck.enable ''
        # Wait for network
        ${pkgs.iputils}/bin/ping -c 1 127.0.0.1 || true
      '';
    };

    # Health check timer (if enabled)
    systemd.timers.traverse-healthcheck = mkIf cfg.healthCheck.enable {
      description = "Traverse health check";
      wantedBy = ["timers.target"];
      timerConfig = {
        OnBootSec = "30s";
        OnUnitActiveSec = "${toString cfg.healthCheck.interval}s";
      };
    };

    # Log rotation
    services.logrotate.settings.traverse = mkIf cfg.logRotation.enable {
      files = "${cfg.dataDir}/logs/*.log";
      frequency = "daily";
      rotate = cfg.logRotation.maxFiles;
      compress = true;
      delaycompress = true;
      missingok = true;
      notifempty = true;
      size = cfg.logRotation.maxSize;
    };

    # Firewall
    networking.firewall.allowedTCPPorts = mkIf cfg.openFirewall [cfg.port];

    # Environment files should be readable by traverse user
    systemd.services.traverse.serviceConfig.EnvironmentFile =
      mkIf (cfg.environmentFiles != [])
      (concatStringsSep " " cfg.environmentFiles);
  };
}
