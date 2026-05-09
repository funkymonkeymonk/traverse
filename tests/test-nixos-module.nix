# Test: Verify NixOS module exists and is valid Nix
# This test checks that the NixOS module can be imported
{pkgs ? import <nixpkgs> {}}: let
  lib = pkgs.lib;

  # Test that the module can be parsed (doesn't check evaluation with full NixOS)
  modulePath = ./../nixos-modules/traverse.nix;
in
  pkgs.runCommand "test-nixos-module"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: NixOS module file exists and is readable..."
    if [ ! -f "${toString modulePath}" ]; then
      echo "FAIL: NixOS module file not found"
      exit 1
    fi
    echo "PASS: NixOS module file exists"

    echo "Testing: NixOS module is valid Nix syntax..."
    if ! ${pkgs.nix}/bin/nix-instantiate --parse "${toString modulePath}" > /dev/null 2>&1; then
      echo "FAIL: NixOS module has syntax errors"
      exit 1
    fi
    echo "PASS: NixOS module syntax is valid"

    echo "Testing: NixOS module imports traverse-options..."
    if ! grep -q "traverse-options" "${toString modulePath}"; then
      echo "FAIL: NixOS module doesn't import traverse-options"
      exit 1
    fi
    echo "PASS: NixOS module imports shared options"

    echo "Testing: NixOS module references systemd..."
    if ! grep -q "systemd" "${toString modulePath}"; then
      echo "FAIL: NixOS module doesn't reference systemd"
      exit 1
    fi
    echo "PASS: NixOS module references systemd"

    echo ""
    echo "✓ All NixOS module tests passed!"

    touch $out
  ''
