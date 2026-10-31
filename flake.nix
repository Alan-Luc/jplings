{
  description = "jplings — Japanese learning system";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

  outputs =
    { nixpkgs, ... }:
    {
      devShells.x86_64-linux.default =
        let
          pkgs = nixpkgs.legacyPackages.x86_64-linux;
        in
        pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            mpv
            pipewire
            docker-compose
          ];
          shellHook = ''
            echo "jplings dev shell — go $(go version | cut -d' ' -f3)"
          '';
          # No cgo dependencies: build a static binary, and do not require gcc.
          CGO_ENABLED = 0;
        };
    };
}
