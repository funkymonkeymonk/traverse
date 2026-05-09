# Traverse package definition
# Builds the Traverse Go binary from source
{
  pkgs ? import <nixpkgs> {},
  lib ? pkgs.lib,
  buildGoModule ? pkgs.buildGoModule,
  fetchFromGitHub ? pkgs.fetchFromGitHub,
}: let
  version = "0.1.0";
in
  buildGoModule rec {
    pname = "traverse";
    inherit version;

    # Since this is a new project, we'll use a placeholder source
    # In production, this would fetch from GitHub or build from local source
    src = ./..;

    # Vendor hash - will need to be updated when go.mod changes
    # Set to null to use vendor directory (if it exists)
    vendorHash = null;

    # Build flags
    ldflags = [
      "-s"
      "-w"
      "-X main.version=${version}"
      "-X main.commit=unknown"
      "-X main.date=unknown"
    ];

    # Sub-packages to build
    subPackages = ["cmd/traverse"];

    # Build dependencies
    nativeBuildInputs = with pkgs; [
      git
    ];

    # Runtime dependencies (if any)
    buildInputs = [];

    # Skip tests during build (tests can be run separately)
    doCheck = false;

    # Install additional files
    postInstall = ''
      mkdir -p $out/share/doc/traverse
      cp -r $src/{README.md,LICENSE,examples} $out/share/doc/traverse/ 2>/dev/null || true
    '';

    meta = with lib; {
      description = "Traverse - MFA secrets proxy with approval workflows";
      longDescription = ''
        Traverse is a self-hosted secrets proxy that adds Multi-Factor
        Authentication (MFA) and approval workflows to arbitrary secret backends.

        Like a rope swing that carries you safely from one cliff to another,
        Traverse bridges the gap between your AI agents and your sensitive secrets,
        with you holding the rope — approving each crossing.
      '';
      homepage = "https://github.com/funkymonkeymonk/traverse";
      license = licenses.mit;
      maintainers = with maintainers; [];
      platforms = platforms.unix;
      mainProgram = "traverse";
    };
  }
