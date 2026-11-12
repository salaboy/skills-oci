{
  description = "CLI tool for packaging and managing AI agent skills as OCI artifacts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems =
        f: nixpkgs.lib.genAttrs supportedSystems (system: f nixpkgs.legacyPackages.${system});

      version = "0-unstable-${self.shortRev or self.dirtyShortRev or "dev"}";
    in
    {
      packages = forAllSystems (pkgs: {
        skills-oci = pkgs.buildGoModule {
          pname = "skills-oci";
          inherit version;

          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./main.go
              ./cmd
              ./pkg
            ];
          };

          # Refresh after go.mod/go.sum changes: set to pkgs.lib.fakeHash and copy the hash `nix build` reports
          vendorHash = "sha256-UgXLCkhQohBa7AtJ71uUclchkavJ73ly3mwXKHzmj5I=";

          env.CGO_ENABLED = 0;

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          meta = {
            description = "CLI tool for packaging and managing AI agent skills as OCI artifacts";
            homepage = "https://github.com/salaboy/skills-oci";
            license = pkgs.lib.licenses.asl20;
            mainProgram = "skills-oci";
          };
        };

        default = self.packages.${pkgs.stdenv.hostPlatform.system}.skills-oci;
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.default;
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          inputsFrom = [ self.packages.${pkgs.stdenv.hostPlatform.system}.default ];
          packages = with pkgs; [
            gopls
            gotools
            golangci-lint
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
