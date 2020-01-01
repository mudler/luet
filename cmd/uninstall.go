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

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <pkg> <pkg2> ...",
	Short: "Uninstall a package or a list of packages",
	Long:  `Uninstall packages`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("system-dbpath", cmd.Flags().Lookup("system-dbpath"))
		viper.BindPFlag("system-target", cmd.Flags().Lookup("system-target"))
	},
	Run: func(cmd *cobra.Command, args []string) {
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

			inst := installer.NewLuetInstaller(LuetCfg.GetGeneral().Concurrency)
			os.MkdirAll(viper.GetString("system-dbpath"), os.ModePerm)
			systemDB := pkg.NewBoltDatabase(filepath.Join(viper.GetString("system-dbpath"), "luet.db"))
			system := &installer.System{Database: systemDB, Target: viper.GetString("system-target")}

			err = inst.Uninstall(pack, system)
			if err != nil {
				Fatal("Error: " + err.Error())
			}
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
	RootCmd.AddCommand(uninstallCmd)
}
