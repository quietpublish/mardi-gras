{
  description = "TUI for Beads issue tracking presented as a parade";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "dev";
      in
      {
        packages = {
          mg = pkgs.buildGoModule {
            pname = "mg";
            inherit version;
            src = ./.;
            vendorHash = "sha256-Ji7ij19/4akUex2gabqrnWFQlr+XvDlHKVg8FxZQ97A=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];

            subPackages = [ "cmd/mg" ];

            meta = with pkgs.lib; {
              description = "TUI for Beads issue tracking presented as a parade";
              homepage = "https://github.com/quiet-publish/mardi-gras";
              license = licenses.mit;
              mainProgram = "mg";
            };
          };
          default = self.packages.${system}.mg;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            golangci-lint
          ];

          shellHook = ''
            echo "mardi-gras dev environment loaded"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
