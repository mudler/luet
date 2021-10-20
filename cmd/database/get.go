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

	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"
	"gopkg.in/yaml.v2"

	"github.com/spf13/cobra"
)

func NewDatabaseGetCommand() *cobra.Command {
	var c = &cobra.Command{
		Use:   "get <package>",
		Short: "Get a package in the system DB as yaml",
		Long: `Get a package in the system database in the YAML format:

		$ luet database get system/foo

To return also files:
		$ luet database get --files system/foo`,
		Args: cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			util.BindSystemFlags(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			showFiles, _ := cmd.Flags().GetBool("files")
			util.SetSystemConfig(util.DefaultContext)

			systemDB := util.DefaultContext.Config.GetSystemDB()

			for _, a := range args {
				pack, err := helpers.ParsePackageStr(a)
				if err != nil {
					continue
				}

				ps, err := systemDB.FindPackages(pack)
				if err != nil {
					continue
				}
				for _, p := range ps {
					y, err := p.Yaml()
					if err != nil {
						continue
					}
					fmt.Println(string(y))
					if showFiles {
						files, err := systemDB.GetPackageFiles(p)
						if err != nil {
							continue
						}
						b, err := yaml.Marshal(files)
						if err != nil {
							continue
						}
						fmt.Println("files:\n" + string(b))
					}
				}
			}
		},
	}
	c.Flags().Bool("files", false, "Show package files.")
	c.Flags().String("system-dbpath", "", "System db path")
	c.Flags().String("system-target", "", "System rootpath")
	c.Flags().String("system-engine", "", "System DB engine")

	return c
}
