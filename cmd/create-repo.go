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

	installer "github.com/mudler/luet/pkg/installer"

	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createrepoCmd = &cobra.Command{
	Use:   "create-repo",
	Short: "Create a luet repository from a build",
	Long:  `Generate and renew repository metadata`,
	Run: func(cmd *cobra.Command, args []string) {

		tree := viper.GetString("tree")
		dst := viper.GetString("output")
		packages := viper.GetString("packages")
		name := viper.GetString("name")
		uri := viper.GetString("uri")
		t := viper.GetString("type")

		repo, err := installer.GenerateRepository(name, uri, t, 1, packages, tree, pkg.NewInMemoryDatabase(false))
		if err != nil {
			Fatal("Error: " + err.Error())
		}
		err = repo.Write(dst)
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
	createrepoCmd.Flags().String("packages", path, "Packages folder (output from build)")
	viper.BindPFlag("packages", createrepoCmd.Flags().Lookup("packages"))

	createrepoCmd.Flags().String("tree", path, "Source luet tree")
	viper.BindPFlag("tree", createrepoCmd.Flags().Lookup("tree"))
	createrepoCmd.Flags().String("output", path, "Destination folder")
	viper.BindPFlag("output", createrepoCmd.Flags().Lookup("output"))
	createrepoCmd.Flags().String("name", "luet", "Repository name")
	viper.BindPFlag("name", createrepoCmd.Flags().Lookup("name"))

	createrepoCmd.Flags().String("uri", path, "Repository uri")
	viper.BindPFlag("uri", createrepoCmd.Flags().Lookup("uri"))

	createrepoCmd.Flags().String("type", "local", "Repository type (local)")
	viper.BindPFlag("type", createrepoCmd.Flags().Lookup("type"))

	RootCmd.AddCommand(createrepoCmd)
}
