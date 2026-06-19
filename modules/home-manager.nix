{ config, lib, ... }:

let
  cfg = config.programs.mkgo;
in
{
  options.programs.mkgo = {
    enable = lib.mkEnableOption "mkgo";

    settings = lib.mkOption {
      type = lib.types.attrs;
      default = {};
    };
  };

  config = lib.mkIf cfg.enable {
    xdg.configFile."mkgo/config.json".text =
      builtins.toJSON cfg.settings;
  };
}