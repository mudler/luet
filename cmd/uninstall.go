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
	"runtime"

	installer "github.com/mudler/luet/pkg/installer"

	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <pkg>",
	Short: "Uninstall a package",
	Long:  `Uninstall packages`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("system-dbpath", cmd.Flags().Lookup("system-dbpath"))
		viper.BindPFlag("system-target", cmd.Flags().Lookup("system-target"))
		viper.BindPFlag("concurrency", cmd.Flags().Lookup("concurrency"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			Fatal("Wrong number of args")
		}

		a := args[0]
		decodepackage, err := regexp.Compile(`^([<>]?\~?=?)((([^\/]+)\/)?(?U)(\S+))(-(\d+(\.\d+)*[a-z]?(_(alpha|beta|pre|rc|p)\d*)*(-r\d+)?))?$`)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
		packageInfo := decodepackage.FindAllStringSubmatch(a, -1)

		category := packageInfo[0][4]
		name := packageInfo[0][5]
		version := packageInfo[0][7]

		inst := installer.NewLuetInstaller(viper.GetInt("concurrency"))
		os.MkdirAll(viper.GetString("system-dbpath"), os.ModePerm)
		systemDB := pkg.NewBoltDatabase(filepath.Join(viper.GetString("system-dbpath"), "luet.db"))
		system := &installer.System{Database: systemDB, Target: viper.GetString("system-target")}
		err = inst.Uninstall(&pkg.DefaultPackage{Name: name, Category: category, Version: version}, system)
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
	uninstallCmd.Flags().String("system-dbpath", path, "System db path")
	uninstallCmd.Flags().String("system-target", path, "System rootpath")
	uninstallCmd.Flags().Int("concurrency", runtime.NumCPU(), "Concurrency")
	RootCmd.AddCommand(uninstallCmd)
}
