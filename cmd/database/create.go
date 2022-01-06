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
	"io/ioutil"

	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/api/core/types"
	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"

	"github.com/spf13/cobra"
)

func NewDatabaseCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <artifact_metadata1.yaml> <artifact_metadata1.yaml>",
		Short: "Insert a package in the system DB",
		Long: `Inserts a package in the system database:

		$ luet database create foo.yaml

"luet database create" injects a package in the system database without actually installing it, use it with caution.

This commands takes multiple yaml input file representing package artifacts, that are usually generated while building packages.

The yaml must contain the package definition, and the file list at least.

For reference, inspect a "metadata.yaml" file generated while running "luet build"`,
		Args: cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {

			systemDB := util.SystemDB(util.DefaultContext.Config)

			for _, a := range args {
				dat, err := ioutil.ReadFile(a)
				if err != nil {
					util.DefaultContext.Fatal("Failed reading ", a, ": ", err.Error())
				}
				art, err := artifact.NewPackageArtifactFromYaml(dat)
				if err != nil {
					util.DefaultContext.Fatal("Failed reading yaml ", a, ": ", err.Error())
				}

				files := art.Files

				// Check if the package is already present
				if p, err := systemDB.FindPackage(art.CompileSpec.GetPackage()); err == nil && p.GetName() != "" {
					util.DefaultContext.Fatal("Package", art.CompileSpec.GetPackage().HumanReadableString(),
						" already present.")
				}

				if _, err := systemDB.CreatePackage(art.CompileSpec.GetPackage()); err != nil {
					util.DefaultContext.Fatal("Failed to create ", a, ": ", err.Error())
				}
				if err := systemDB.SetPackageFiles(&types.PackageFile{PackageFingerprint: art.CompileSpec.GetPackage().GetFingerPrint(), Files: files}); err != nil {
					util.DefaultContext.Fatal("Failed setting package files for ", a, ": ", err.Error())
				}

				util.DefaultContext.Info(art.CompileSpec.GetPackage().HumanReadableString(), " created")
			}

		},
	}

}
