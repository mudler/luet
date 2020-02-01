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
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/marcsauter/single"
	config "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	repo "github.com/mudler/luet/pkg/repository"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var Verbose bool

const (
	LuetCLIVersion = "0.5-dev"
	LuetEnvPrefix  = "LUET"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "luet",
	Short:   "Package manager for the XXth century!",
	Long:    `Package manager which uses containers to build packages`,
	Version: LuetCLIVersion,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := LoadConfig(config.LuetCfg)
		if err != nil {
			Fatal("failed to load configuration:", err.Error())
		}
	},
}

func LoadConfig(c *config.LuetConfig) error {
	// If a config file is found, read it in.
	if err := c.Viper.ReadInConfig(); err != nil {
		Warning(err)
	}

	err := c.Viper.Unmarshal(&config.LuetCfg)
	if err != nil {
		return err
	}

	Debug("Using config file:", c.Viper.ConfigFileUsed())

	NewSpinner()

	if c.GetLogging().Path != "" {
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
	// XXX: This is mostly from scratch images.
	if os.Getenv("LUET_NOLOCK") != "true" {
		s := single.New("luet")
		if err := s.CheckLock(); err != nil && err == single.ErrAlreadyRunning {
			Fatal("another instance of the app is already running, exiting")
		} else if err != nil {
			// Another error occurred, might be worth handling it as well
			Fatal("failed to acquire exclusive app lock:", err.Error())
		}
		defer s.TryUnlock()
	}
	if err := RootCmd.Execute(); err != nil {
		Error(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	pflags := RootCmd.PersistentFlags()
	pflags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.luet.yaml)")
	pflags.BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
	pflags.Bool("fatal", false, "Enables Warnings to exit")

	u, err := user.Current()
	if err != nil {
		Fatal("failed to retrieve user identity:", err.Error())
	}
	sameOwner := false
	if u.Uid == "0" {
		sameOwner = true
	}
	pflags.Bool("same-owner", sameOwner, "Maintain same owner on uncompress.")
	pflags.Int("concurrency", runtime.NumCPU(), "Concurrency")

	config.LuetCfg.Viper.BindPFlag("general.same_owner", pflags.Lookup("same-owner"))
	config.LuetCfg.Viper.BindPFlag("general.debug", pflags.Lookup("verbose"))
	config.LuetCfg.Viper.BindPFlag("general.concurrency", pflags.Lookup("concurrency"))
	config.LuetCfg.Viper.BindPFlag("general.fatal_warnings", pflags.Lookup("fatal"))
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
		Info(">>> cfgFile: ", cfgFile)
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
