# Test: Verify package definition exists and is importable
# This test verifies the Traverse Go package can be evaluated
{pkgs ? import <nixpkgs> {}}: let
  # Import the package definition
  traversePkg = import ../packages/traverse.nix {inherit pkgs;};
in
  pkgs.runCommand "test-package-definition"
  {
    buildInputs = [pkgs.nix];
  }
  ''
    set -e

    echo "Testing: Package definition is not empty..."
    if [ -z "${toString traversePkg}" ]; then
      echo "FAIL: Package definition is empty"
      exit 1
    fi
    echo "PASS: Package definition exists"

    echo "Testing: Package has expected meta attributes..."
    # Check that meta attributes are defined
    metaDescription = "${toString traversePkg.meta.description or "missing"}"
    if [ "$metaDescription" = "missing" ]; then
      echo "FAIL: Package missing meta.description"
      exit 1
    fi
    echo "PASS: Package has meta.description"

    echo ""
    echo "✓ All package definition tests passed!"

    touch $out
  ''
