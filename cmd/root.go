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

	"github.com/mudler/luet/cmd/util"
	bus "github.com/mudler/luet/pkg/api/core/bus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var Verbose bool

const (
	LuetCLIVersion = "0.20.8"
	LuetEnvPrefix  = "LUET"
)

var license = []string{
	"Luet Copyright (C) 2019-2021 Ettore Di Giacinto",
	"This program comes with ABSOLUTELY NO WARRANTY.",
	"This is free software, and you are welcome to redistribute it under certain conditions.",
}

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
		err := util.InitContext(util.DefaultContext)
		if err != nil {
			util.DefaultContext.Error("failed to load configuration:", err.Error())
		}
		util.DisplayVersionBanner(util.DefaultContext, util.IntroScreen, version, license)

		// Initialize tmpdir prefix. TODO: Move this with LoadConfig
		// directly on sub command to ensure the creation only when it's
		// needed.
		err = util.DefaultContext.Config.GetSystem().InitTmpDir()
		if err != nil {
			util.DefaultContext.Fatal("failed on init tmp basedir:", err.Error())
		}

		viper.BindPFlag("plugin", cmd.Flags().Lookup("plugin"))

		plugin := viper.GetStringSlice("plugin")

		bus.Manager.Initialize(util.DefaultContext, plugin...)
		if len(bus.Manager.Plugins) != 0 {
			util.DefaultContext.Info(":lollipop:Enabled plugins:")
			for _, p := range bus.Manager.Plugins {
				util.DefaultContext.Info(fmt.Sprintf("\t:arrow_right: %s (at %s)", p.Name, p.Executable))
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Cleanup all tmp directories used by luet
		err := util.DefaultContext.Config.GetSystem().CleanupTmpDir()
		if err != nil {
			util.DefaultContext.Warning("failed on cleanup tmpdir:", err.Error())
		}
	},
	SilenceErrors: true,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	util.HandleLock(util.DefaultContext)

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	util.InitViper(util.DefaultContext, RootCmd)
}
