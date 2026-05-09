# NixOS module entry point
# Imports and re-exports the Traverse NixOS module
{
  config,
  lib,
  pkgs,
  ...
}: {
  imports = [
    ./traverse.nix
  ];
}
