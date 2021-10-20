// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	extensions "github.com/mudler/cobra-extensions"
	"github.com/mudler/luet/pkg/api/core/types"

	helpers "github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	LuetEnvPrefix = "LUET"
)

var cfgFile string

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	setDefaults(viper.GetViper())
	// Luet support these priorities on read configuration file:
	// - command line option (if available)
	// - $PWD/.luet.yaml
	// - $HOME/.luet.yaml
	// - /etc/luet/luet.yaml
	//
	// Note: currently a single viper instance support only one config name.

	viper.SetEnvPrefix(LuetEnvPrefix)
	viper.SetConfigType("yaml")

	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Retrieve pwd directory
		pwdDir, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		homeDir := helpers.GetHomeDir()

		if fileHelper.Exists(filepath.Join(pwdDir, ".luet.yaml")) || (homeDir != "" && fileHelper.Exists(filepath.Join(homeDir, ".luet.yaml"))) {
			viper.AddConfigPath(".")
			if homeDir != "" {
				viper.AddConfigPath(homeDir)
			}
			viper.SetConfigName(".luet")
		} else {
			viper.SetConfigName("luet")
			viper.AddConfigPath("/etc/luet")
		}
	}

	viper.AutomaticEnv() // read in environment variables that match

	// Create EnvKey Replacer for handle complex structure
	replacer := strings.NewReplacer(".", "__")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetTypeByDefaultValue(true)
	// If a config file is found, read it in.
	viper.ReadInConfig()

}

// InitContext inits the context by parsing the configurations from viper
// this is meant to be run before each command to be able to parse any override from
// the CLI/ENV
func InitContext(ctx *types.Context) (err error) {

	err = viper.Unmarshal(&ctx.Config)
	if err != nil {
		return
	}

	// Inits the context with the configurations loaded
	// It reads system repositories, sets logging, and all the
	// context which is required to perform luet actions
	err = ctx.Init()
	if err != nil {
		return
	}

	// no_spinner is not mapped in our configs
	ctx.NoSpinner = viper.GetBool("no_spinner")
	return
}

func setDefaults(viper *viper.Viper) {
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.enable_logfile", false)
	viper.SetDefault("logging.path", "/var/log/luet.log")
	viper.SetDefault("logging.json_format", false)
	viper.SetDefault("logging.enable_emoji", true)
	viper.SetDefault("logging.color", true)

	viper.SetDefault("general.concurrency", runtime.NumCPU())
	viper.SetDefault("general.debug", false)
	viper.SetDefault("general.show_build_output", false)
	viper.SetDefault("general.fatal_warnings", false)

	u, err := user.Current()
	// os/user doesn't work in from scratch environments
	if err != nil || (u != nil && u.Uid == "0") {
		viper.SetDefault("general.same_owner", true)
	} else {
		viper.SetDefault("general.same_owner", false)
	}

	viper.SetDefault("system.database_engine", "boltdb")
	viper.SetDefault("system.database_path", "/var/cache/luet")
	viper.SetDefault("system.rootfs", "/")
	viper.SetDefault("system.tmpdir_base", filepath.Join(os.TempDir(), "tmpluet"))
	viper.SetDefault("system.pkgs_cache_path", "packages")

	viper.SetDefault("repos_confdir", []string{"/etc/luet/repos.conf.d"})
	viper.SetDefault("config_protect_confdir", []string{"/etc/luet/config.protect.d"})
	viper.SetDefault("config_protect_skip", false)
	// TODO: Set default to false when we are ready for migration.
	viper.SetDefault("config_from_host", true)
	viper.SetDefault("cache_repositories", []string{})
	viper.SetDefault("system_repositories", []string{})
	viper.SetDefault("finalizer_envs", make(map[string]string))

	viper.SetDefault("solver.type", "")
	viper.SetDefault("solver.rate", 0.7)
	viper.SetDefault("solver.discount", 1.0)
	viper.SetDefault("solver.max_attempts", 9000)
}

// InitViper inits a new viper
// this is meant to be run just once at beginning to setup the root command
func InitViper(ctx *types.Context, RootCmd *cobra.Command) {
	cobra.OnInitialize(initConfig)
	pflags := RootCmd.PersistentFlags()
	pflags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.luet.yaml)")
	pflags.BoolP("debug", "d", false, "verbose output")
	pflags.Bool("fatal", false, "Enables Warnings to exit")
	pflags.Bool("enable-logfile", false, "Enable log to file")
	pflags.Bool("no-spinner", false, "Disable spinner.")
	pflags.Bool("color", ctx.Config.GetLogging().Color, "Enable/Disable color.")
	pflags.Bool("emoji", ctx.Config.GetLogging().EnableEmoji, "Enable/Disable emoji.")
	pflags.Bool("skip-config-protect", ctx.Config.ConfigProtectSkip,
		"Disable config protect analysis.")
	pflags.StringP("logfile", "l", ctx.Config.GetLogging().Path,
		"Logfile path. Empty value disable log to file.")
	pflags.StringSlice("plugin", []string{}, "A list of runtime plugins to load")

	// os/user doesn't work in from scratch environments.
	// Check if i can retrieve user informations.
	_, err := user.Current()
	if err != nil {
		ctx.Warning("failed to retrieve user identity:", err.Error())
	}
	pflags.Bool("same-owner", ctx.Config.GetGeneral().SameOwner, "Maintain same owner on uncompress.")
	pflags.Int("concurrency", runtime.NumCPU(), "Concurrency")

	viper.BindPFlag("logging.color", pflags.Lookup("color"))
	viper.BindPFlag("logging.enable_emoji", pflags.Lookup("emoji"))
	viper.BindPFlag("logging.enable_logfile", pflags.Lookup("enable-logfile"))
	viper.BindPFlag("logging.path", pflags.Lookup("logfile"))

	viper.BindPFlag("general.concurrency", pflags.Lookup("concurrency"))
	viper.BindPFlag("general.debug", pflags.Lookup("debug"))
	viper.BindPFlag("general.fatal_warnings", pflags.Lookup("fatal"))
	viper.BindPFlag("general.same_owner", pflags.Lookup("same-owner"))
	viper.BindPFlag("plugin", pflags.Lookup("plugin"))

	// Currently I maintain this only from cli.
	viper.BindPFlag("no_spinner", pflags.Lookup("no-spinner"))
	viper.BindPFlag("config_protect_skip", pflags.Lookup("skip-config-protect"))

	// Extensions must be binary with the "luet-" prefix to be able to be shown in the help.
	// we also accept extensions in the relative path where luet is being started, "extensions/"
	exts := extensions.Discover("luet", "extensions")
	for _, ex := range exts {
		cobraCmd := ex.CobraCommand()
		RootCmd.AddCommand(cobraCmd)
	}
}
