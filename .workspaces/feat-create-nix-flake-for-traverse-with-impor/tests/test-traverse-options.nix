# Test: Verify shared options schema exports expected options
# This test verifies that lib/traverse-options.nix exports the correct option definitions
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Import the options schema
  optionsSchema = import ../lib/traverse-options.nix {inherit lib;};
in
  pkgs.runCommand "test-traverse-options"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: traverse-options exports enable option..."
    if [ -z "${toString (optionsSchema.enable or "")}" ]; then
      echo "FAIL: enable option not exported"
      exit 1
    fi
    echo "PASS: enable option exists"

    echo "Testing: traverse-options exports package option..."
    if [ -z "${toString (optionsSchema.package or "")}" ]; then
      echo "FAIL: package option not exported"
      exit 1
    fi
    echo "PASS: package option exists"

    echo "Testing: traverse-options exports settings option..."
    if [ -z "${toString (optionsSchema.settings or "")}" ]; then
      echo "FAIL: settings option not exported"
      exit 1
    fi
    echo "PASS: settings option exists"

    echo "Testing: traverse-options exports environmentFiles option..."
    if [ -z "${toString (optionsSchema.environmentFiles or "")}" ]; then
      echo "FAIL: environmentFiles option not exported"
      exit 1
    fi
    echo "PASS: environmentFiles option exists"

    echo "Testing: traverse-options exports port option..."
    if [ -z "${toString (optionsSchema.port or "")}" ]; then
      echo "FAIL: port option not exported"
      exit 1
    fi
    echo "PASS: port option exists"

    echo "Testing: traverse-options exports host option..."
    if [ -z "${toString (optionsSchema.host or "")}" ]; then
      echo "FAIL: host option not exported"
      exit 1
    fi
    echo "PASS: host option exists"

    echo ""
    echo "✓ All traverse-options tests passed!"

    touch $out
  ''
