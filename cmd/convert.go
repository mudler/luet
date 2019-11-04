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
	"runtime"

	. "github.com/mudler/luet/pkg/logger"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/mudler/luet/pkg/tree/builder/gentoo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "convert other package manager tree into luet",
	Long:  `Parses external PM and produces a luet parsable tree`,
	Run: func(cmd *cobra.Command, args []string) {

		t := viper.GetString("type")
		c := viper.GetInt("concurrency")

		if len(args) != 2 {
			Fatal("Incorrect number of arguments")
		}

		input := args[0]
		output := args[1]
		Info("Converting trees from " + input + " [" + t + "]")

		var builder tree.Parser
		switch t {
		case "gentoo":
			builder = gentoo.NewGentooBuilder(&gentoo.SimpleEbuildParser{}, c, gentoo.InMemory)
		default: // dup
			builder = gentoo.NewGentooBuilder(&gentoo.SimpleEbuildParser{}, c, gentoo.InMemory)
		}

		packageTree, err := builder.Generate(input)
		if err != nil {
			Fatal("Error: " + err.Error())
		}

		defer packageTree.GetPackageSet().Clean()
		Info("Tree generated")

		generalRecipe := tree.NewGeneralRecipe()
		Info("Saving generated tree to " + output)

		generalRecipe.WithTree(packageTree)
		err = generalRecipe.Save(output)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {
	convertCmd.Flags().String("type", "gentoo", "source type")
	viper.BindPFlag("type", convertCmd.Flags().Lookup("type"))
	convertCmd.Flags().Int("concurrency", runtime.NumCPU(), "Concurrency")
	viper.BindPFlag("concurrency", convertCmd.Flags().Lookup("concurrency"))
	RootCmd.AddCommand(convertCmd)
}
