# Test: Verify Darwin module evaluates correctly
# This test checks that the Darwin module can be imported and evaluated
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Evaluate the Darwin module with test configuration
  evalResult = lib.evalModules {
    modules = [
      ../darwin-modules/traverse.nix
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
  pkgs.runCommand "test-darwin-module-eval"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: Darwin module evaluates without errors..."
    # The fact that evalResult was computed means evaluation succeeded
    echo "PASS: Darwin module evaluates successfully"

    echo "Testing: Launchd service is created when enabled..."
    # Check that launchd service config is generated
    serviceConfig = "${toString evalResult.config.launchd.daemons.traverse or "missing"}"
    if [ "$serviceConfig" = "missing" ]; then
      echo "FAIL: launchd service not generated"
      exit 1
    fi
    echo "PASS: Launchd service is created"

    echo "Testing: Service user is referenced..."
    userConfig = "${toString evalResult.config.users.users.traverse or "missing"}"
    if [ "$userConfig" = "missing" ]; then
      echo "FAIL: traverse user not referenced"
      exit 1
    fi
    echo "PASS: Service user is referenced"

    echo ""
    echo "✓ All Darwin module evaluation tests passed!"

    touch $out
  ''
