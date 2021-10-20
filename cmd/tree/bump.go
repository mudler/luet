// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package cmd_tree

import (
	"fmt"

	"github.com/mudler/luet/cmd/util"
	spectooling "github.com/mudler/luet/pkg/spectooling"
	tree "github.com/mudler/luet/pkg/tree"
	version "github.com/mudler/luet/pkg/versioner"

	"github.com/spf13/cobra"
)

func NewTreeBumpCommand() *cobra.Command {

	var ans = &cobra.Command{
		Use:   "bump [OPTIONS]",
		Short: "Bump a new package build version.",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			df, _ := cmd.Flags().GetString("definition-file")
			if df == "" {
				util.DefaultContext.Fatal("Mandatory definition.yaml path missing.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			spec, _ := cmd.Flags().GetString("definition-file")
			toStdout, _ := cmd.Flags().GetBool("to-stdout")
			pkgVersion, _ := cmd.Flags().GetString("pkg-version")
			pack, err := tree.ReadDefinitionFile(spec)
			if err != nil {
				util.DefaultContext.Fatal(err.Error())
			}

			if pkgVersion != "" {
				validator := &version.WrappedVersioner{}
				err := validator.Validate(pkgVersion)
				if err != nil {
					util.DefaultContext.Fatal("Invalid version string: " + err.Error())
				}
				pack.SetVersion(pkgVersion)
			} else {
				// Retrieve version build section with Gentoo parser
				err = pack.BumpBuildVersion()
				if err != nil {
					util.DefaultContext.Fatal("Error on increment build version: " + err.Error())
				}
			}
			if toStdout {
				data, err := spectooling.NewDefaultPackageSanitized(&pack).Yaml()
				if err != nil {
					util.DefaultContext.Fatal("Error on yaml conversion: " + err.Error())
				}
				fmt.Println(string(data))
			} else {

				err = tree.WriteDefinitionFile(&pack, spec)
				if err != nil {
					util.DefaultContext.Fatal("Error on write definition file: " + err.Error())
				}

				fmt.Printf("Bumped package %s/%s-%s.\n", pack.Category, pack.Name, pack.Version)
			}
		},
	}

	ans.Flags().StringP("pkg-version", "p", "", "Set a specific package version")
	ans.Flags().StringP("definition-file", "f", "", "Path of the definition to bump.")
	ans.Flags().BoolP("to-stdout", "o", false, "Bump package to output.")

	return ans
}
