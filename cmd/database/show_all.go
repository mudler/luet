// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"fmt"
	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"os"
)

func NewDatabaseShowAllCommand() *cobra.Command {
	var c = &cobra.Command{
		Use:   "get-all-installed",
		Short: "Show all installed packages in the system DB as yaml",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			systemDB := util.SystemDB(util.DefaultContext.Config)
			var packages []*types.Package

			packs := systemDB.GetPackages()
			for _, p := range packs {
				pack, _ := systemDB.GetPackage(p)
				packages = append(packages, pack)
			}
			marshal, err := yaml.Marshal(packages)
			if err != nil {
				return
			}
			fmt.Println(string(marshal))
			output, _ := cmd.Flags().GetString("output")
			f, err := os.Create(output)
			if err != nil {
				fmt.Printf("Error creating file: %s\n", err)
				return
			}
			_, err = f.WriteString(string(marshal))
			if err != nil {
				fmt.Printf("Error writing file: %s\n", err)
				return
			}
			err = f.Close()
			if err != nil {
				fmt.Printf("Error closing file: %s\n", err)
				return
			}
		},
	}

	c.Flags().String("output", "", "Save output to given file.")
	return c
}
