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
	installer "github.com/mudler/luet/pkg/installer"

	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var installCmd = &cobra.Command{
	Use:   "install <pkg1> <pkg2> ...",
	Short: "Install a package",
	Long: `Installs one or more packages without asking questions:

	$ luet install -y utils/busybox utils/yq ...
	
To install only deps of a package:
	
	$ luet install --onlydeps utils/busybox ...
	
To not install deps of a package:
	
	$ luet install --nodeps utils/busybox ...

To force install a package:
	
	$ luet install --force utils/busybox ...
`,
	Aliases: []string{"i"},
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var toInstall pkg.Packages

		for _, a := range args {
			pack, err := helpers.ParsePackageStr(a)
			if err != nil {
				util.DefaultContext.Fatal("Invalid package string ", a, ": ", err.Error())
			}
			toInstall = append(toInstall, pack)
		}

		force := viper.GetBool("force")
		nodeps := viper.GetBool("nodeps")
		onlydeps := viper.GetBool("onlydeps")
		yes := viper.GetBool("yes")
		downloadOnly, _ := cmd.Flags().GetBool("download-only")
		relax, _ := cmd.Flags().GetBool("relax")

		util.DefaultContext.Debug("Solver", util.DefaultContext.Config.Solver.CompactString())

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 util.DefaultContext.Config.General.Concurrency,
			SolverOptions:               util.DefaultContext.Config.Solver,
			NoDeps:                      nodeps,
			Force:                       force,
			OnlyDeps:                    onlydeps,
			PreserveSystemEssentialData: true,
			DownloadOnly:                downloadOnly,
			Ask:                         !yes,
			Relaxed:                     relax,
			PackageRepositories:         util.DefaultContext.Config.SystemRepositories,
			Context:                     util.DefaultContext,
		})

		system := &installer.System{
			Database: util.DefaultContext.Config.GetSystemDB(),
			Target:   util.DefaultContext.Config.System.Rootfs,
		}
		err := inst.Install(toInstall, system)
		if err != nil {
			util.DefaultContext.Fatal("Error: " + err.Error())
		}
	},
}

func init() {

	installCmd.Flags().Bool("nodeps", false, "Don't consider package dependencies (harmful!)")
	installCmd.Flags().Bool("relax", false, "Relax installation constraints")

	installCmd.Flags().Bool("onlydeps", false, "Consider **only** package dependencies")
	installCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")
	installCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	installCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	installCmd.Flags().Bool("download-only", false, "Download only")
	installCmd.Flags().StringArray("finalizer-env", []string{},
		"Set finalizer environment in the format key=value.")

	RootCmd.AddCommand(installCmd)
}
