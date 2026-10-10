{
  description = "kit — an agent toolkit for the terminal (Nix flake)";

  # flake.lock is deliberately not committed. The only inputs are nixpkgs and
  # the pinned release below, and an unlocked nixpkgs is what keeps the build
  # working: kit asks for a recent Go, and a lock file pinned to an old
  # nixpkgs would eventually hold a Go that refuses to compile the module
  # graph. Run `nix flake lock` locally if you want a lock for your own use.
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    # The release that `nix profile install github:mark3labs/kit` installs.
    #
    # A flake input cannot say "latest tag". A bare `github:mark3labs/kit`
    # resolves to the default branch, so an unpinned input would install
    # unreleased master instead of a release. The `nix-release-pin` job in
    # .github/workflows/release.yml rewrites the url below after every tagged
    # release, so the pin on the default branch always names the newest tag.
    #
    # Keep the `# nix:kit-release-tag` marker on the url line:
    # scripts/bump-flake-release-pin.sh matches on it.
    kit-release = {
      url = "github:mark3labs/kit/v0.126.0"; # nix:kit-release-tag
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      kit-release,
      ...
    }:
    let
      inherit (nixpkgs) lib;

      # Version of the release pinned by inputs.kit-release above, without the
      # "v". scripts/bump-flake-release-pin.sh writes both lines from one tag
      # and refuses to run when they disagree.
      #
      # It cannot be derived from the input: a `flake = false` input arrives in
      # the outputs as a bare store path, so the tag in the url is not visible
      # here.
      version = "0.126.0"; # nix:kit-release-version

      # The platforms goreleaser builds. Keep them in sync with .goreleaser.yaml.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      eachSystem =
        f:
        lib.genAttrs systems (
          system:
          f {
            inherit system;
            pkgs = import nixpkgs { inherit system; };
          }
        );

      # The Go series a tree needs: "go 1.27.0" -> "127".
      goSeriesOf =
        src:
        let
          directives = lib.filter (lib.hasPrefix "go ") (
            lib.splitString "\n" (builtins.readFile (src + "/go.mod"))
          );
        in
        lib.concatStringsSep "" (
          lib.take 2 (
            lib.splitVersion (
              if directives == [ ] then "1.24" else lib.removePrefix "go " (lib.head directives)
            )
          )
        );

      # nixpkgs specialises buildGoModule per Go series (buildGo127Module), and
      # the helper it picks by default is not always new enough for the go.mod
      # of the tree being built: the build sets GOTOOLCHAIN=local, so Go refuses
      # to download a newer toolchain from inside the sandbox. Ask for the
      # helper of the series the tree asks for, and fall back to the default
      # helper when nixpkgs has not packaged it yet.
      goModuleBuilder =
        pkgs: src:
        let
          name = "buildGo${goSeriesOf src}Module";
        in
        if builtins.hasAttr name pkgs then pkgs.${name} else pkgs.buildGoModule;

      # The compiler itself, for the dev shell, same series as the build.
      goCompiler =
        pkgs: src:
        let
          name = "go_${goSeriesOf src}";
        in
        if builtins.hasAttr name pkgs then pkgs.${name} else pkgs.go;

      mkKit =
        {
          pkgs,
          src,
          version,
        }:
        (goModuleBuilder pkgs src) {
          pname = "kit";
          inherit src version;

          # Only the CLI. The rest of the tree holds libraries, tests and
          # example extensions that an install does not need.
          subPackages = [ "cmd/kit" ];

          # Fetch the modules in a fixed-output derivation and vendor them for
          # the build. The hash pins the module tree that the pinned release's
          # go.sum asks for, so it has to change with every release that ships
          # different dependencies. The `nix-release-pin` job refreshes it with
          # scripts/update-flake-vendor-hash.sh after each release, and the
          # same script fixes it by hand when deps move between releases:
          #
          #   scripts/update-flake-vendor-hash.sh
          vendorHash = "sha256-iJyqXNvIeIgQ9q+c9uaZIpS0yMTJUWbGhByL/nUw7bc=";

          # The same static binary goreleaser ships, so the profile needs no
          # wrapper and does not depend on the glibc of the build machine.
          env.CGO_ENABLED = "0";

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          doCheck = false;

          meta = {
            description = "kit — an agent toolkit for the terminal";
            homepage = "https://github.com/mark3labs/kit";
            license = lib.licenses.mit;
            mainProgram = "kit";
            platforms = systems;
          };
        };

      # The pinned release, built for one system.
      releaseFor =
        pkgs:
        mkKit {
          inherit pkgs version;
          src = kit-release;
        };
    in
    {
      packages = eachSystem (
        {
          pkgs,
          ...
        }:
        rec {
          kit = releaseFor pkgs;
          default = kit;
        }
      );

      apps = eachSystem (
        {
          pkgs,
          ...
        }:
        rec {
          kit = {
            type = "app";
            program = "${releaseFor pkgs}/bin/kit";
          };
          default = kit;
        }
      );

      # `nix flake check` then builds the release the same way a user installs
      # it, and the binary is left in the store to run against.
      checks = eachSystem ({ pkgs, ... }: { kit = releaseFor pkgs; });

      devShells = eachSystem (
        {
          pkgs,
          ...
        }:
        {
          default = pkgs.mkShell {
            packages = [
              # The Go the tree asks for, plus the usual Go tooling. Build and
              # run kit from here with `task build` / `go build ./cmd/kit` —
              # the shell deliberately ships no prebuilt kit, because the nix
              # build vendors the *pinned release's* modules behind a hash,
              # and a working tree with newer deps would fight it.
              (goCompiler pkgs self)
              pkgs.gopls
              pkgs.golangci-lint
            ];
          };
        }
      );

      formatter = eachSystem ({ pkgs, ... }: pkgs.nixfmt-rfc-style);

      # For projects that prefer a nixpkgs-style package:
      #   inputs.kit.url = "github:mark3labs/kit";
      #   inputs.kit.overlays.default
      overlays.default = final: _prev: { kit = releaseFor final; };
    };
}
