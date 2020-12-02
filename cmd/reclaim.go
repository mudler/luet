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
	"path/filepath"

	installer "github.com/mudler/luet/pkg/installer"

	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
)

var reclaimCmd = &cobra.Command{
	Use:   "reclaim",
	Short: "Reclaim packages to Luet database from available repositories",
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("force", cmd.Flags().Lookup("force"))
	},
	Long: `Reclaim tries to find association between packages in the online repositories and the system one.

	$ luet reclaim

It scans the target file system, and if finds a match with a package available in the repositories, it marks as installed in the system database.
`,
	Run: func(cmd *cobra.Command, args []string) {
		var systemDB pkg.PackageDatabase

		// This shouldn't be necessary, but we need to unmarshal the repositories to a concrete struct, thus we need to port them back to the Repositories type
		repos := installer.Repositories{}
		for _, repo := range LuetCfg.SystemRepositories {
			if !repo.Enable {
				continue
			}
			r := installer.NewSystemRepository(repo)
			repos = append(repos, r)
		}

		force := LuetCfg.Viper.GetBool("force")

		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 LuetCfg.GetGeneral().Concurrency,
			Force:                       force,
			PreserveSystemEssentialData: true,
		})
		inst.Repositories(repos)

		if LuetCfg.GetSystem().DatabaseEngine == "boltdb" {
			systemDB = pkg.NewBoltDatabase(
				filepath.Join(LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
		} else {
			systemDB = pkg.NewInMemoryDatabase(true)
		}
		system := &installer.System{Database: systemDB, Target: LuetCfg.GetSystem().Rootfs}
		err := inst.Reclaim(system)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	reclaimCmd.Flags().String("system-dbpath", path, "System db path")
	reclaimCmd.Flags().String("system-target", path, "System rootpath")
	reclaimCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")

	RootCmd.AddCommand(reclaimCmd)
}
