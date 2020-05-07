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
	LuetCLIVersion = "0.8-dev"
	LuetEnvPrefix  = "LUET"
)

// Build time and commit information.
//
// ⚠️ WARNING: should only be set by "-ldflags".
var (
	BuildTime   string
	BuildCommit string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "luet",
	Short:   "Package manager for the XXth century!",
	Long:    `Package manager which uses containers to build packages`,
	Version: fmt.Sprintf("%s-g%s %s", LuetCLIVersion, BuildCommit, BuildTime),
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

	Debug("Using config file:", c.Viper.ConfigFileUsed())

	NewSpinner()

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

	if err := RootCmd.Execute(); err != nil {
		if len(os.Args) > 0 {
			for _, c := range RootCmd.Commands() {
				if c.Name() == os.Args[1] {
					os.Exit(-1) // Something failed
				}
			}
			// Try to load a bin from path.
			helpers.Exec("luet-"+os.Args[1], os.Args[1:], os.Environ())
		}
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
	pflags.StringP("logfile", "l", config.LuetCfg.GetLogging().Path,
		"Logfile path. Empty value disable log to file.")

	sameOwner := false
	u, err := user.Current()
	// os/user doesn't work in from scratch environments
	if err != nil {
		Warning("failed to retrieve user identity:", err.Error())
		sameOwner = true
	}
	if u != nil && u.Uid == "0" {
		sameOwner = true
	}
	pflags.Bool("same-owner", sameOwner, "Maintain same owner on uncompress.")
	pflags.Int("concurrency", runtime.NumCPU(), "Concurrency")

	config.LuetCfg.Viper.BindPFlag("logging.enable_logfile", pflags.Lookup("enable-logfile"))
	config.LuetCfg.Viper.BindPFlag("logging.path", pflags.Lookup("logfile"))

	config.LuetCfg.Viper.BindPFlag("general.concurrency", pflags.Lookup("concurrency"))
	config.LuetCfg.Viper.BindPFlag("general.debug", pflags.Lookup("debug"))
	config.LuetCfg.Viper.BindPFlag("general.fatal_warnings", pflags.Lookup("fatal"))
	config.LuetCfg.Viper.BindPFlag("general.same_owner", pflags.Lookup("same-owner"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		Error(err)
		os.Exit(1)
	}
	viper.SetEnvPrefix(LuetEnvPrefix)
	viper.SetConfigType("yaml")
	viper.SetConfigName(".luet") // name of config file (without extension)
	if cfgFile != "" {           // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(dir)
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
		viper.AddConfigPath("/etc/luet")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// Create EnvKey Replacer for handle complex structure
	replacer := strings.NewReplacer(".", "__")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetTypeByDefaultValue(true)
}
