# Test: Verify Darwin module exists and is valid Nix
# This test checks that the Darwin module can be imported
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Test that the module can be parsed
  modulePath = ./../darwin-modules/traverse.nix;
in
  pkgs.runCommand "test-darwin-module"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: Darwin module file exists and is readable..."
    if [ ! -f "${toString modulePath}" ]; then
      echo "FAIL: Darwin module file not found"
      exit 1
    fi
    echo "PASS: Darwin module file exists"

    echo "Testing: Darwin module is valid Nix syntax..."
    if ! ${pkgs.nix}/bin/nix-instantiate --parse "${toString modulePath}" > /dev/null 2>&1; then
      echo "FAIL: Darwin module has syntax errors"
      exit 1
    fi
    echo "PASS: Darwin module syntax is valid"

    echo "Testing: Darwin module imports traverse-options..."
    if ! grep -q "traverse-options" "${toString modulePath}"; then
      echo "FAIL: Darwin module doesn't import traverse-options"
      exit 1
    fi
    echo "PASS: Darwin module imports shared options"

    echo "Testing: Darwin module references launchd..."
    if ! grep -q "launchd" "${toString modulePath}"; then
      echo "FAIL: Darwin module doesn't reference launchd"
      exit 1
    fi
    echo "PASS: Darwin module references launchd"

    echo ""
    echo "✓ All Darwin module tests passed!"

    touch $out
  ''
