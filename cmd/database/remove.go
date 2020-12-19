// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package cmd_database

import (
	. "github.com/mudler/luet/pkg/logger"

	helpers "github.com/mudler/luet/cmd/helpers"
	. "github.com/mudler/luet/pkg/config"

	"github.com/spf13/cobra"
)

func NewDatabaseRemoveCommand() *cobra.Command {
	var ans = &cobra.Command{
		Use:   "remove [package1] [package2] ...",
		Short: "Remove a package from the system DB (forcefully - you normally don't want to do that)",
		Long: `Removes a package in the system database without actually uninstalling it:

		$ luet database remove foo/bar

This commands takes multiple packages as arguments and prunes their entries from the system database.
`,
		Args: cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
			LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))

		},
		Run: func(cmd *cobra.Command, args []string) {
			systemDB := LuetCfg.GetSystemDB()

			for _, a := range args {
				pack, err := helpers.ParsePackageStr(a)
				if err != nil {
					Fatal("Invalid package string ", a, ": ", err.Error())
				}

				if err := systemDB.RemovePackage(pack); err != nil {
					Fatal("Failed removing ", a, ": ", err.Error())
				}

				if err := systemDB.RemovePackageFiles(pack); err != nil {
					Fatal("Failed removing files for ", a, ": ", err.Error())
				}
			}

		},
	}

	return ans
}
