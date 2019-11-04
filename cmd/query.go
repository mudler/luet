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
	"log"

	. "github.com/mudler/luet/pkg/logger"

	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var queryCmd = &cobra.Command{
	Use:   "query install <pkg>",
	Short: "query other package manager tree into luet",
	Long:  `Parses external PM and produces a luet parsable tree`,
	Run: func(cmd *cobra.Command, args []string) {

		input := viper.GetString("input")

		if len(args) != 4 {
			log.Fatalln("Incorrect number of arguments")
		}

		generalRecipe := tree.NewGeneralRecipe()
		fmt.Println("Loading generated tree from " + input)

		err := generalRecipe.Load(input)
		if err != nil {
			Fatal("Error: " + err.Error())
		}

		defer generalRecipe.Tree().GetPackageSet().Clean()

		t := args[0]
		v := args[1]
		version := args[2]
		cat := args[3]
		switch t {
		case "install":
			// XXX: pack needs to be the same which is present in world.
			// Tree caches generated world when using FindPackage
			pack, err := generalRecipe.Tree().FindPackage(&pkg.DefaultPackage{Category: cat, Name: v, Version: version})
			if err != nil {
				Fatal("Error: " + err.Error())
			}

			fmt.Println("Install query from " + input + " [" + v + "]")
			world, err := generalRecipe.Tree().World()
			if err != nil {
				Fatal("Error: " + err.Error())
			}
			fmt.Println(">>> World")
			for _, packss := range world {
				packss.Explain()
			}
			s := solver.NewSolver([]pkg.Package{}, world)
			solution, err := s.Install([]pkg.Package{pack})
			if err != nil {
				Fatal("Error: " + err.Error())
			}
			fmt.Println(">>> Solution")

			for _, assertion := range solution {
				assertion.Explain()
			}
		}

	},
}

func init() {
	queryCmd.Flags().String("input", "", "source folder")
	viper.BindPFlag("input", queryCmd.Flags().Lookup("input"))
	RootCmd.AddCommand(queryCmd)
}
