# Test: Verify shared options schema exports expected options
# This test verifies that lib/traverse-options.nix exports the correct option definitions
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Import the options schema
  optionsSchema = import ./../lib/traverse-options.nix {inherit lib;};

  # Helper to check if option exists
  hasOption = optName: (optionsSchema.${optName} or null) != null;
in
  pkgs.runCommand "test-traverse-options"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: traverse-options exports enable option..."
    if [ "${
      if hasOption "enable"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: enable option not exported"
      exit 1
    fi
    echo "PASS: enable option exists"

    echo "Testing: traverse-options exports package option..."
    if [ "${
      if hasOption "package"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: package option not exported"
      exit 1
    fi
    echo "PASS: package option exists"

    echo "Testing: traverse-options exports settings option..."
    if [ "${
      if hasOption "settings"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: settings option not exported"
      exit 1
    fi
    echo "PASS: settings option exists"

    echo "Testing: traverse-options exports environmentFiles option..."
    if [ "${
      if hasOption "environmentFiles"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: environmentFiles option not exported"
      exit 1
    fi
    echo "PASS: environmentFiles option exists"

    echo "Testing: traverse-options exports port option..."
    if [ "${
      if hasOption "port"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: port option not exported"
      exit 1
    fi
    echo "PASS: port option exists"

    echo "Testing: traverse-options exports host option..."
    if [ "${
      if hasOption "host"
      then "true"
      else "false"
    }" != "true" ]; then
      echo "FAIL: host option not exported"
      exit 1
    fi
    echo "PASS: host option exists"

    echo "Testing: traverse-options is valid Nix syntax..."
    if ! ${pkgs.nix}/bin/nix-instantiate --parse ${./../lib/traverse-options.nix} > /dev/null 2>&1; then
      echo "FAIL: traverse-options has syntax errors"
      exit 1
    fi
    echo "PASS: traverse-options syntax is valid"

    echo ""
    echo "✓ All traverse-options tests passed!"

    touch $out
  ''
