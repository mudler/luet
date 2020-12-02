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
	. "github.com/mudler/luet/cmd/database"

	"github.com/spf13/cobra"
)

var databaseGroupCmd = &cobra.Command{
	Use:   "database [command] [OPTIONS]",
	Short: "Manage system database (dangerous commands ahead!)",
	Long: `Allows to manipulate Luet internal database of installed packages. Use with caution!

Removing packages by hand from the database can result in a broken system, and thus it's not reccomended.
`,
}

func init() {
	RootCmd.AddCommand(databaseGroupCmd)

	databaseGroupCmd.AddCommand(
		NewDatabaseCreateCommand(),
		NewDatabaseRemoveCommand(),
	)
}
