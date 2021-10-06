// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/marcsauter/single"
	bus "github.com/mudler/luet/pkg/bus"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	extensions "github.com/mudler/cobra-extensions"
	config "github.com/mudler/luet/pkg/config"
	helpers "github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	repo "github.com/mudler/luet/pkg/repository"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var Verbose bool
var LockedCommands = []string{"install", "uninstall", "upgrade"}

const (
	LuetCLIVersion = "0.17.12"
	LuetEnvPrefix  = "LUET"
	license        = `
	Luet Copyright (C) 2019-2021 Ettore Di Giacinto
	This program comes with ABSOLUTELY NO WARRANTY.
	This is free software, and you are welcome to redistribute it
	under certain conditions.
	`
)

// Build time and commit information.
//
// ⚠️ WARNING: should only be set by "-ldflags".
var (
	BuildTime   string
	BuildCommit string
)

func version() string {
	return fmt.Sprintf("%s-g%s %s", LuetCLIVersion, BuildCommit, BuildTime)
}

var noBannerCommands = []string{"search", "exec", "tree", "database", "box", "cleanup"}

func displayVersionBanner() {
	display := true
	if len(os.Args) > 1 {
		for _, c := range noBannerCommands {
			if os.Args[1] == c {
				display = false
			}
		}
	}
	if display {
		Info("Luet version", version())
		Info(license)
	}
}

func handleLock() {
	if os.Getenv("LUET_NOLOCK") != "true" {
		if len(os.Args) > 1 {
			for _, lockedCmd := range LockedCommands {
				if os.Args[1] == lockedCmd {
					s := single.New("luet")
					if err := s.CheckLock(); err != nil && err == single.ErrAlreadyRunning {
						Fatal("another instance of the app is already running, exiting")
					} else if err != nil {
						// Another error occurred, might be worth handling it as well
						Fatal("failed to acquire exclusive app lock:", err.Error())
					}
					defer s.TryUnlock()
					break
				}
			}
		}
	}
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "luet",
	Short: "Container based package manager",
	Long: `Luet is a single-binary package manager based on containers to build packages.
	
To install a package:

	$ luet install package

To search for a package in the repositories:

$ luet search package

To list all packages installed in the system:

	$ luet search --installed .

To show hidden packages:

	$ luet search --hidden package

To build a package, from a tree definition:

	$ luet build --tree tree/path package
	
`,
	Version: version(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		err := LoadConfig(config.LuetCfg)
		if err != nil {
			Fatal("failed to load configuration:", err.Error())
		}
		// Initialize tmpdir prefix. TODO: Move this with LoadConfig
		// directly on sub command to ensure the creation only when it's
		// needed.
		err = config.LuetCfg.GetSystem().InitTmpDir()
		if err != nil {
			Fatal("failed on init tmp basedir:", err.Error())
		}

		viper.BindPFlag("plugin", cmd.Flags().Lookup("plugin"))

		plugin := viper.GetStringSlice("plugin")

		bus.Manager.Initialize(plugin...)
		if len(bus.Manager.Plugins) != 0 {
			Info(":lollipop:Enabled plugins:")
			for _, p := range bus.Manager.Plugins {
				Info("\t:arrow_right:", p.Name)
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Cleanup all tmp directories used by luet
		err := config.LuetCfg.GetSystem().CleanupTmpDir()
		if err != nil {
			Warning("failed on cleanup tmpdir:", err.Error())
		}
	},
	SilenceErrors: true,
}

func LoadConfig(c *config.LuetConfig) error {
	// If a config file is found, read it in.
	c.Viper.ReadInConfig()

	err := c.Viper.Unmarshal(&config.LuetCfg)
	if err != nil {
		return err
	}

	noSpinner := c.Viper.GetBool("no_spinner")

	InitAurora()
	if !noSpinner {
		NewSpinner()
	}

	Debug("Using config file:", c.Viper.ConfigFileUsed())

	if c.GetLogging().EnableLogFile && c.GetLogging().Path != "" {
		// Init zap logger
		err = ZapLogger()
		if err != nil {
			return err
		}
	}

	// Load repositories
	err = repo.LoadRepositories(c)
	if err != nil {
		return err
	}

	return nil
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

	handleLock()
	displayVersionBanner()

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	pflags := RootCmd.PersistentFlags()
	pflags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.luet.yaml)")
	pflags.BoolP("debug", "d", false, "verbose output")
	pflags.Bool("fatal", false, "Enables Warnings to exit")
	pflags.Bool("enable-logfile", false, "Enable log to file")
	pflags.Bool("no-spinner", false, "Disable spinner.")
	pflags.Bool("color", config.LuetCfg.GetLogging().Color, "Enable/Disable color.")
	pflags.Bool("emoji", config.LuetCfg.GetLogging().EnableEmoji, "Enable/Disable emoji.")
	pflags.Bool("skip-config-protect", config.LuetCfg.ConfigProtectSkip,
		"Disable config protect analysis.")
	pflags.StringP("logfile", "l", config.LuetCfg.GetLogging().Path,
		"Logfile path. Empty value disable log to file.")
	pflags.StringSlice("plugin", []string{}, "A list of runtime plugins to load")

	// os/user doesn't work in from scratch environments.
	// Check if i can retrieve user informations.
	_, err := user.Current()
	if err != nil {
		Warning("failed to retrieve user identity:", err.Error())
	}
	pflags.Bool("same-owner", config.LuetCfg.GetGeneral().SameOwner, "Maintain same owner on uncompress.")
	pflags.Int("concurrency", runtime.NumCPU(), "Concurrency")

	config.LuetCfg.Viper.BindPFlag("logging.color", pflags.Lookup("color"))
	config.LuetCfg.Viper.BindPFlag("logging.enable_emoji", pflags.Lookup("emoji"))
	config.LuetCfg.Viper.BindPFlag("logging.enable_logfile", pflags.Lookup("enable-logfile"))
	config.LuetCfg.Viper.BindPFlag("logging.path", pflags.Lookup("logfile"))

	config.LuetCfg.Viper.BindPFlag("general.concurrency", pflags.Lookup("concurrency"))
	config.LuetCfg.Viper.BindPFlag("general.debug", pflags.Lookup("debug"))
	config.LuetCfg.Viper.BindPFlag("general.fatal_warnings", pflags.Lookup("fatal"))
	config.LuetCfg.Viper.BindPFlag("general.same_owner", pflags.Lookup("same-owner"))
	config.LuetCfg.Viper.BindPFlag("plugin", pflags.Lookup("plugin"))

	// Currently I maintain this only from cli.
	config.LuetCfg.Viper.BindPFlag("no_spinner", pflags.Lookup("no-spinner"))
	config.LuetCfg.Viper.BindPFlag("config_protect_skip", pflags.Lookup("skip-config-protect"))

	// Extensions must be binary with the "luet-" prefix to be able to be shown in the help.
	// we also accept extensions in the relative path where luet is being started, "extensions/"
	exts := extensions.Discover("luet", "extensions")
	for _, ex := range exts {
		cobraCmd := ex.CobraCommand()
		RootCmd.AddCommand(cobraCmd)
	}

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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
			Error(err)
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
}
