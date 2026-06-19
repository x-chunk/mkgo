{
  description = "A development environment for a mkgo";

  inputs = {
    nixpkgs.url = "github:Nixos/nixpkgs/nixos-unstable";
    # Helper library for multi-system support
    flake-utils.url = "github:numtide/flake-utils";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, flake-utils, home-manager }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        # Development shell configuration
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            gotools
          ];

          shellHook = ''
            export GOPATH="$HOME/.go"
            export PATH="$GOPATH/bin:$PATH"

            echo "⚡ mkgo development environment loaded via Nix Flakes!"
            go version
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "mkgo";
          version = "0.1.0";

          src = ./.;
          vendorHash = null;
        };
      }
    ) // {
      homeManagerModules.default =
        import ./modules/home-manager.nix;
    };
}