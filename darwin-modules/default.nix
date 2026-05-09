# Darwin module entry point
# Imports and re-exports the Traverse Darwin module
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
