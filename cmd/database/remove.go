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
	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"

	"github.com/spf13/cobra"
)

func NewDatabaseRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [package1] [package2] ...",
		Short: "Remove a package from the system DB (forcefully - you normally don't want to do that)",
		Long: `Removes a package in the system database without actually uninstalling it:

		$ luet database remove foo/bar

This commands takes multiple packages as arguments and prunes their entries from the system database.
`,
		Args: cobra.OnlyValidArgs,

		Run: func(cmd *cobra.Command, args []string) {

			systemDB := util.SystemDB(util.DefaultContext.Config)

			for _, a := range args {
				pack, err := helpers.ParsePackageStr(a)
				if err != nil {
					util.DefaultContext.Fatal("Invalid package string ", a, ": ", err.Error())
				}

				if err := systemDB.RemovePackage(pack); err != nil {
					util.DefaultContext.Fatal("Failed removing ", a, ": ", err.Error())
				}

				if err := systemDB.RemovePackageFiles(pack); err != nil {
					util.DefaultContext.Fatal("Failed removing files for ", a, ": ", err.Error())
				}
			}

		},
	}

}
