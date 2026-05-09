# Test suite for Traverse Nix flake
# Aggregates all tests for CI and local validation
{pkgs ? import <nixpkgs> {}}: {
  # Structure tests verify file existence and basic flake properties
  flake-structure = import ./test-flake-structure.nix {inherit pkgs;};

  # Options schema tests verify lib/traverse-options.nix exports
  traverse-options = import ./test-traverse-options.nix {inherit pkgs;};

  # Package definition tests
  package-definition = import ./test-package-definition.nix {inherit pkgs;};

  # Module evaluation tests
  nixos-module = import ./test-nixos-module.nix {inherit pkgs;};
  darwin-module = import ./test-darwin-module.nix {inherit pkgs;};
}
