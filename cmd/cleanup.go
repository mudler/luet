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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/cmd/util"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean packages cache.",
	Long:  `remove downloaded packages tarballs and clean cache directory`,
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		var cleaned int = 0
		util.SetSystemConfig(util.DefaultContext)
		// Check if cache dir exists
		if fileHelper.Exists(util.DefaultContext.Config.GetSystem().GetSystemPkgsCacheDirPath()) {

			files, err := ioutil.ReadDir(util.DefaultContext.Config.GetSystem().GetSystemPkgsCacheDirPath())
			if err != nil {
				util.DefaultContext.Fatal("Error on read cachedir ", err.Error())
			}

			for _, file := range files {

				if util.DefaultContext.Config.GetGeneral().Debug {
					util.DefaultContext.Info("Removing ", file.Name())
				}

				err := os.RemoveAll(
					filepath.Join(util.DefaultContext.Config.GetSystem().GetSystemPkgsCacheDirPath(), file.Name()))
				if err != nil {
					util.DefaultContext.Fatal("Error on removing", file.Name())
				}
				cleaned++
			}
		}

		util.DefaultContext.Info(fmt.Sprintf("Cleaned: %d files from %s", cleaned, util.DefaultContext.Config.GetSystem().GetSystemPkgsCacheDirPath()))

	},
}

func init() {
	cleanupCmd.Flags().String("system-dbpath", "", "System db path")
	cleanupCmd.Flags().String("system-target", "", "System rootpath")
	cleanupCmd.Flags().String("system-engine", "", "System DB engine")
	RootCmd.AddCommand(cleanupCmd)
}
