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
	"path/filepath"

	installer "github.com/mudler/luet/pkg/installer"

	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <pkg1> <pkg2> ...",
	Short: "Install a package",
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
	},
	Long: `Install packages in parallel`,
	Run: func(cmd *cobra.Command, args []string) {
		var toInstall []pkg.Package
		var systemDB pkg.PackageDatabase

		for _, a := range args {
			gp, err := _gentoo.ParsePackageStr(a)
			if err != nil {
				Fatal("Invalid package string ", a, ": ", err.Error())
			}

			if gp.Version == "" {
				gp.Version = "0"
				gp.Condition = _gentoo.PkgCondGreaterEqual
			}

			pack := &pkg.DefaultPackage{
				Name: gp.Name,
				Version: fmt.Sprintf("%s%s%s",
					pkg.PkgSelectorConditionFromInt(gp.Condition.Int()).String(),
					gp.Version,
					gp.VersionSuffix,
				),
				Category: gp.Category,
				Uri:      make([]string, 0),
			}
			toInstall = append(toInstall, pack)
		}

		// This shouldn't be necessary, but we need to unmarshal the repositories to a concrete struct, thus we need to port them back to the Repositories type
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

		if LuetCfg.GetSystem().DatabaseEngine == "boltdb" {
			systemDB = pkg.NewBoltDatabase(
				filepath.Join(helpers.GetSystemRepoDatabaseDirPath(), "luet.db"))
		} else {
			systemDB = pkg.NewInMemoryDatabase(true)
		}
		system := &installer.System{Database: systemDB, Target: LuetCfg.GetSystem().Rootfs}
		err := inst.Install(toInstall, system)
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
	installCmd.Flags().String("system-dbpath", path, "System db path")
	installCmd.Flags().String("system-target", path, "System rootpath")

	RootCmd.AddCommand(installCmd)
}
