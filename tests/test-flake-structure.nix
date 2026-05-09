# Test: Verify Nix flake structure exists
# This test checks that all required Nix files exist and are importable
{pkgs ? import <nixpkgs> {}}:
pkgs.runCommand "test-flake-structure"
{
  buildInputs = [pkgs.nix pkgs.gnugrep];
}
''
  set -e

  REPO_ROOT="${toString ./..}"

  # Test: flake.nix exists
  echo "Testing: flake.nix exists..."
  if [ ! -f "$REPO_ROOT/flake.nix" ]; then
    echo "FAIL: flake.nix not found"
    exit 1
  fi
  echo "PASS: flake.nix exists"

  # Test: lib/traverse-options.nix exists
  echo "Testing: lib/traverse-options.nix exists..."
  if [ ! -f "$REPO_ROOT/lib/traverse-options.nix" ]; then
    echo "FAIL: lib/traverse-options.nix not found"
    exit 1
  fi
  echo "PASS: lib/traverse-options.nix exists"

  # Test: packages/traverse.nix exists
  echo "Testing: packages/traverse.nix exists..."
  if [ ! -f "$REPO_ROOT/packages/traverse.nix" ]; then
    echo "FAIL: packages/traverse.nix not found"
    exit 1
  fi
  echo "PASS: packages/traverse.nix exists"

  # Test: nixos-modules/traverse.nix exists
  echo "Testing: nixos-modules/traverse.nix exists..."
  if [ ! -f "$REPO_ROOT/nixos-modules/traverse.nix" ]; then
    echo "FAIL: nixos-modules/traverse.nix not found"
    exit 1
  fi
  echo "PASS: nixos-modules/traverse.nix exists"

  # Test: nixos-modules/default.nix exists
  echo "Testing: nixos-modules/default.nix exists..."
  if [ ! -f "$REPO_ROOT/nixos-modules/default.nix" ]; then
    echo "FAIL: nixos-modules/default.nix not found"
    exit 1
  fi
  echo "PASS: nixos-modules/default.nix exists"

  # Test: darwin-modules/traverse.nix exists
  echo "Testing: darwin-modules/traverse.nix exists..."
  if [ ! -f "$REPO_ROOT/darwin-modules/traverse.nix" ]; then
    echo "FAIL: darwin-modules/traverse.nix not found"
    exit 1
  fi
  echo "PASS: darwin-modules/traverse.nix exists"

  # Test: darwin-modules/default.nix exists
  echo "Testing: darwin-modules/default.nix exists..."
  if [ ! -f "$REPO_ROOT/darwin-modules/default.nix" ]; then
    echo "FAIL: darwin-modules/default.nix not found"
    exit 1
  fi
  echo "PASS: darwin-modules/default.nix exists"

  # Test: Flake has proper description
  echo "Testing: flake.nix has description..."
  if ! grep -q 'description.*=' "$REPO_ROOT/flake.nix"; then
    echo "FAIL: flake.nix missing description"
    exit 1
  fi
  echo "PASS: flake.nix has description"

  # All tests passed
  echo ""
  echo "✓ All flake structure tests passed!"

  touch $out
''
