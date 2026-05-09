{
  description = "Traverse - MFA secrets proxy with approval workflows";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-utils = {
      url = "github:numtide/flake-utils";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    ...
  }: let
    # Supported systems
    supportedSystems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];

    # Helper function to generate outputs for each system
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

    # Nixpkgs with overlays
    nixpkgsFor = forAllSystems (
      system:
        import nixpkgs {
          inherit system;
          overlays = [self.overlays.default];
        }
    );
  in {
    # Overlays - makes traverse package available
    overlays = {
      default = final: prev: {
        traverse = final.callPackage ./packages/traverse.nix {};
      };
    };

    # Packages for each system
    packages = forAllSystems (
      system: let
        pkgs = nixpkgsFor.${system};
      in {
        traverse = pkgs.traverse;
        default = pkgs.traverse;
      }
    );

    # NixOS modules - can be imported by other flakes
    nixosModules = {
      traverse = import ./nixos-modules/traverse.nix;
      default = self.nixosModules.traverse;
    };

    # Darwin modules - can be imported by other flakes
    darwinModules = {
      traverse = import ./darwin-modules/traverse.nix;
      default = self.darwinModules.traverse;
    };

    # Library functions - exported for reuse
    lib = {
      # Export the traverse options schema for type checking
      traverseOptions = import ./lib/traverse-options.nix;
    };

    # Development shells
    devShells = forAllSystems (
      system: let
        pkgs = nixpkgsFor.${system};
      in {
        default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            nixpkgs-fmt
            alejandra
          ];

          shellHook = ''
            echo "🧗 Traverse development shell"
            echo "Run 'go build ./cmd/traverse' to build"
          '';
        };
      }
    );

    # Checks - run with 'nix flake check'
    checks = forAllSystems (
      system: let
        pkgs = nixpkgsFor.${system};
      in
        import ./tests {inherit pkgs;}
    );

    # Formatter - run with 'nix fmt'
    formatter = forAllSystems (
      system:
        nixpkgsFor.${system}.alejandra
    );

    # Legacy attributes for compatibility
    defaultPackage = forAllSystems (
      system:
        self.packages.${system}.traverse
    );

    defaultNixosConfiguration = self.nixosModules.default;
  };
}
