{ pkgs, ... }:
{
  # go.mod: go 1.26.0
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  packages = with pkgs; [ git gnumake ];
}
