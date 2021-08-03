// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/mudler/luet/pkg/config"
	config "github.com/mudler/luet/pkg/config"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean packages cache.",
	Long:  `remove downloaded packages tarballs and clean cache directory`,
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("installed", cmd.Flags().Lookup("installed"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var cleaned int = 0
		dbpath := LuetCfg.Viper.GetString("system.database_path")
		rootfs := config.LuetCfg.Viper.GetString("system.rootfs")
		engine := config.LuetCfg.Viper.GetString("system.database_engine")

		LuetCfg.System.DatabaseEngine = engine
		LuetCfg.System.DatabasePath = dbpath
		LuetCfg.System.SetRootFS(rootfs)
		// Check if cache dir exists
		if fileHelper.Exists(LuetCfg.GetSystem().GetSystemPkgsCacheDirPath()) {

			files, err := ioutil.ReadDir(LuetCfg.GetSystem().GetSystemPkgsCacheDirPath())
			if err != nil {
				Fatal("Error on read cachedir ", err.Error())
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}

				if LuetCfg.GetGeneral().Debug {
					Info("Removing ", file.Name())
				}

				err := os.RemoveAll(
					filepath.Join(LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), file.Name()))
				if err != nil {
					Fatal("Error on removing", file.Name())
				}
				cleaned++
			}
		}

		Info("Cleaned: ", cleaned, "packages.")

	},
}

func init() {
	cleanupCmd.Flags().String("system-dbpath", "", "System db path")
	cleanupCmd.Flags().String("system-target", "", "System rootpath")
	cleanupCmd.Flags().String("system-engine", "", "System DB engine")
	RootCmd.AddCommand(cleanupCmd)
}
