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
	"github.com/mudler/luet/cmd/util"
	installer "github.com/mudler/luet/pkg/installer"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reclaimCmd = &cobra.Command{
	Use:   "reclaim",
	Short: "Reclaim packages to Luet database from available repositories",
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
		viper.BindPFlag("force", cmd.Flags().Lookup("force"))
	},
	Long: `Reclaim tries to find association between packages in the online repositories and the system one.

	$ luet reclaim

It scans the target file system, and if finds a match with a package available in the repositories, it marks as installed in the system database.
`,
	Run: func(cmd *cobra.Command, args []string) {
		util.SetSystemConfig(util.DefaultContext)

		force := viper.GetBool("force")

		util.DefaultContext.Debug("Solver", util.DefaultContext.Config.GetSolverOptions().CompactString())

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 util.DefaultContext.Config.GetGeneral().Concurrency,
			Force:                       force,
			PreserveSystemEssentialData: true,
			PackageRepositories:         util.DefaultContext.Config.SystemRepositories,
			Context:                     util.DefaultContext,
		})

		system := &installer.System{Database: util.DefaultContext.Config.GetSystemDB(), Target: util.DefaultContext.Config.GetSystem().Rootfs}
		err := inst.Reclaim(system)
		if err != nil {
			util.DefaultContext.Fatal("Error: " + err.Error())
		}
	},
}

func init() {

	reclaimCmd.Flags().String("system-dbpath", "", "System db path")
	reclaimCmd.Flags().String("system-target", "", "System rootpath")
	reclaimCmd.Flags().String("system-engine", "", "System DB engine")

	reclaimCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")

	RootCmd.AddCommand(reclaimCmd)
}
