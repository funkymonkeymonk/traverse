# Darwin (macOS) module for Traverse
# Provides launchd service configuration for Traverse on macOS
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
    # Create traverse user (Darwin style)
    users.users.traverse = {
      description = "Traverse service user";
      uid = mkDefault 900;
      gid = mkDefault config.users.groups.traverse.gid;
      home = cfg.dataDir;
      createHome = true;
      shell = "/bin/bash";
    };

    users.groups.traverse = {
      gid = mkDefault 900;
    };

    # Create data directory
    system.activationScripts.postActivation.text = ''
      mkdir -p '${cfg.dataDir}'
      mkdir -p '${cfg.dataDir}/logs'
      chown -R ${cfg.user}:${cfg.group} '${cfg.dataDir}'
      chmod 750 '${cfg.dataDir}'
    '';

    # Launchd service
    launchd.daemons.traverse = {
      description = "Traverse - MFA secrets proxy with approval workflows";

      serviceConfig = {
        Label = "org.nixos.traverse";
        ProgramArguments = [
          "${pkgs.bash}/bin/bash"
          "-c"
          ''
            ${envFileScript}
            exec ${cfg.package or defaultPackage}/bin/traverse server --config ${configFile}
          ''
        ];

        RunAtLoad = true;
        KeepAlive = true;

        # User and group
        UserName = cfg.user;
        GroupName = cfg.group;

        # Working directory
        WorkingDirectory = cfg.dataDir;

        # Environment
        EnvironmentVariables = {
          PATH = "${pkgs.coreutils}/bin:${pkgs.bash}/bin";
        };

        # Logging
        StandardOutPath = "${cfg.dataDir}/logs/traverse.log";
        StandardErrorPath = "${cfg.dataDir}/logs/traverse.error.log";

        # Resource limits
        SoftResourceLimits = {
          NumberOfFiles = 65536;
        };
        HardResourceLimits = {
          NumberOfFiles = 65536;
        };

        # Throttling
        ThrottleInterval = 5;
      };
    };

    # Log rotation (using newsyslog on Darwin)
    environment.etc."newsyslog.d/traverse.conf".text = mkIf cfg.logRotation.enable ''
      # Traverse log rotation
      ${cfg.dataDir}/logs/traverse.log    ${cfg.user}:${cfg.group}   640  ${toString cfg.logRotation.maxFiles}    *    ${cfg.logRotation.maxSize}
      ${cfg.dataDir}/logs/traverse.error.log    ${cfg.user}:${cfg.group}   640  ${toString cfg.logRotation.maxFiles}    *    ${cfg.logRotation.maxSize}
    '';

    # Note: Firewall on Darwin must be configured separately
    # Users should use the macOS firewall or pf configuration
  };
}
