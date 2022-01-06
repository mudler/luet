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
package cmd

import (
	"github.com/mudler/luet/pkg/api/core/types"
	installer "github.com/mudler/luet/pkg/installer"

	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reinstallCmd = &cobra.Command{
	Use:   "reinstall <pkg1> <pkg2> <pkg3>",
	Short: "reinstall a set of packages",
	Long: `Reinstall a group of packages in the system:

	$ luet reinstall -y system/busybox shells/bash system/coreutils ...
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		viper.BindPFlag("for", cmd.Flags().Lookup("for"))

		viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var toUninstall types.Packages
		var toAdd types.Packages

		force := viper.GetBool("force")
		onlydeps := viper.GetBool("onlydeps")
		yes := viper.GetBool("yes")

		downloadOnly, _ := cmd.Flags().GetBool("download-only")
		installed, _ := cmd.Flags().GetBool("installed")

		util.DefaultContext.Debug("Solver", util.DefaultContext.Config.Solver.CompactString())

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 util.DefaultContext.Config.General.Concurrency,
			SolverOptions:               util.DefaultContext.Config.Solver,
			NoDeps:                      true,
			Force:                       force,
			OnlyDeps:                    onlydeps,
			PreserveSystemEssentialData: true,
			Ask:                         !yes,
			DownloadOnly:                downloadOnly,
			Context:                     util.DefaultContext,
			PackageRepositories:         util.DefaultContext.Config.SystemRepositories,
		})

		system := &installer.System{Database: util.SystemDB(util.DefaultContext.Config), Target: util.DefaultContext.Config.System.Rootfs}

		if installed {
			for _, p := range system.Database.World() {
				toUninstall = append(toUninstall, p)
				c := p.Clone()
				c.SetVersion(">=0")
				toAdd = append(toAdd, c)
			}
		} else {
			for _, a := range args {
				pack, err := helpers.ParsePackageStr(a)
				if err != nil {
					util.DefaultContext.Fatal("Invalid package string ", a, ": ", err.Error())
				}
				toUninstall = append(toUninstall, pack)
				toAdd = append(toAdd, pack)
			}
		}

		err := inst.Swap(toUninstall, toAdd, system)
		if err != nil {
			util.DefaultContext.Fatal("Error: " + err.Error())
		}
	},
}

func init() {
	reinstallCmd.Flags().Bool("onlydeps", false, "Consider **only** package dependencies")
	reinstallCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")
	reinstallCmd.Flags().Bool("installed", false, "Reinstall installed packages")
	reinstallCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	reinstallCmd.Flags().Bool("download-only", false, "Download only")

	RootCmd.AddCommand(reinstallCmd)
}
