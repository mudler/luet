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

var installCmd = &cobra.Command{
	Use:   "install <pkg1> <pkg2> ...",
	Short: "Install a package",
	Long:  `Install packages in parallel`,
	Run: func(cmd *cobra.Command, args []string) {
		c := installer.Repositories{}
		err := viper.UnmarshalKey("system-repositories", &c)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
		var toInstall []pkg.Package

		for _, a := range args {
			decodepackage, err := regexp.Compile(`^([<>]?\~?=?)((([^\/]+)\/)?(?U)(\S+))(-(\d+(\.\d+)*[a-z]?(_(alpha|beta|pre|rc|p)\d*)*(-r\d+)?))?$`)
			if err != nil {
				Fatal("Error: " + err.Error())
			}
			packageInfo := decodepackage.FindAllStringSubmatch(a, -1)

			category := packageInfo[0][4]
			name := packageInfo[0][5]
			version := packageInfo[0][7]
			toInstall = append(toInstall, &pkg.DefaultPackage{Name: name, Category: category, Version: version})

		}

		inst := installer.NewLuetInstaller(viper.GetInt("concurrency"))

		inst.Repositories(c)

		systemDB := pkg.NewBoltDatabase(filepath.Join(viper.GetString("system-dbpath"), "luet"))
		system := &installer.System{Database: systemDB, Target: viper.GetString("system-target")}
		err = inst.Install(toInstall, system)
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
	viper.BindPFlag("system-dbpath", installCmd.Flags().Lookup("system-dbpath"))

	installCmd.Flags().String("system-target", path, "System rootpath")
	viper.BindPFlag("system-target", installCmd.Flags().Lookup("system-target"))

	installCmd.Flags().Int("concurrency", runtime.NumCPU(), "Concurrency")
	viper.BindPFlag("concurrency", installCmd.Flags().Lookup("concurrency"))

	RootCmd.AddCommand(installCmd)
}
