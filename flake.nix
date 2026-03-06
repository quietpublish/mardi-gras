{
  description = "BubbleTea TUI for Beads issue tracking";

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
            vendorHash = "sha256-t6079kO6VE8Rn28TzTWbtIQSxFKv6/+n3ff8t/ZIDMc=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];

            subPackages = [ "cmd/mg" ];

            meta = with pkgs.lib; {
              description = "BubbleTea TUI for Beads issue tracking";
              homepage = "https://github.com/matt-wright86/mardi-gras";
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
