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

	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrades the system",
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", installCmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", installCmd.Flags().Lookup("system-target"))
	},
	Long: `Upgrades packages in parallel`,
	Run: func(cmd *cobra.Command, args []string) {
		var systemDB pkg.PackageDatabase

		repos := installer.Repositories{}
		for _, repo := range LuetCfg.SystemRepositories {
			if !repo.Enable {
				continue
			}

			r := installer.NewSystemRepository(repo)
			repos = append(repos, r)
		}

		inst := installer.NewLuetInstaller(LuetCfg.GetGeneral().Concurrency)
		inst.Repositories(repos)
		_, err := inst.SyncRepositories(false)
		if err != nil {
			Fatal("Error: " + err.Error())
		}

		if LuetCfg.GetSystem().DatabaseEngine == "boltdb" {
			systemDB = pkg.NewBoltDatabase(
				filepath.Join(LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
		} else {
			systemDB = pkg.NewInMemoryDatabase(true)
		}
		system := &installer.System{Database: systemDB, Target: LuetCfg.GetSystem().Rootfs}
		err = inst.Upgrade(system)
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
	upgradeCmd.Flags().String("system-dbpath", path, "System db path")
	upgradeCmd.Flags().String("system-target", path, "System rootpath")

	RootCmd.AddCommand(upgradeCmd)
}
