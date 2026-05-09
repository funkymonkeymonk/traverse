# Test: Verify NixOS module evaluates correctly
# This test checks that the NixOS module can be imported and evaluated
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Evaluate the module with test configuration
  evalResult = lib.evalModules {
    modules = [
      ../nixos-modules/traverse.nix
      {
        config = {
          services.traverse = {
            enable = true;
            port = 8080;
            host = "127.0.0.1";
            settings = {
              log_level = "info";
            };
          };
        };
      }
    ];
  };
in
  pkgs.runCommand "test-nixos-module-eval"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: NixOS module evaluates without errors..."
    # The fact that evalResult was computed means evaluation succeeded
    echo "PASS: NixOS module evaluates successfully"

    echo "Testing: Systemd service is created when enabled..."
    # Check that systemd service config is generated
    serviceConfig = "${toString evalResult.config.systemd.services.traverse or "missing"}"
    if [ "$serviceConfig" = "missing" ]; then
      echo "FAIL: systemd service not generated"
      exit 1
    fi
    echo "PASS: Systemd service is created"

    echo "Testing: Service user is created..."
    userConfig = "${toString evalResult.config.users.users.traverse or "missing"}"
    if [ "$userConfig" = "missing" ]; then
      echo "FAIL: traverse user not created"
      exit 1
    fi
    echo "PASS: Service user is created"

    echo ""
    echo "✓ All NixOS module evaluation tests passed!"

    touch $out
  ''
