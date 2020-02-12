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
	"regexp"

	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search packages",
	Long:  `Search for installed and available packages`,
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		viper.BindPFlag("installed", cmd.Flags().Lookup("installed"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var systemDB pkg.PackageDatabase

		if len(args) != 1 {
			Fatal("Wrong number of arguments (expected 1)")
		}
		installed := viper.GetBool("installed")

		if !installed {

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
			synced, err := inst.SyncRepositories(false)
			if err != nil {
				Fatal("Error: " + err.Error())
			}

			Info("--- Search results: ---")

			matches := synced.Search(args[0])
			for _, m := range matches {
				Info(":package:", m.Package.GetCategory(), m.Package.GetName(),
					m.Package.GetVersion(), "repository:", m.Repo.GetName())
			}
		} else {

			if LuetCfg.GetSystem().DatabaseEngine == "boltdb" {
				systemDB = pkg.NewBoltDatabase(
					filepath.Join(LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
			} else {
				systemDB = pkg.NewInMemoryDatabase(true)
			}
			system := &installer.System{Database: systemDB, Target: LuetCfg.GetSystem().Rootfs}
			var term = regexp.MustCompile(args[0])

			for _, k := range system.Database.GetPackages() {
				pack, err := system.Database.GetPackage(k)
				if err == nil && term.MatchString(pack.GetName()) {
					Info(":package:", pack.GetCategory(), pack.GetName(), pack.GetVersion())
				}
			}
		}

	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	searchCmd.Flags().String("system-dbpath", path, "System db path")
	searchCmd.Flags().String("system-target", path, "System rootpath")
	searchCmd.Flags().Bool("installed", false, "Search between system packages")
	RootCmd.AddCommand(searchCmd)
}
