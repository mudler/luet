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
	"runtime"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var buildCmd = &cobra.Command{
	Use:   "build <cat> <name> <version>",
	Short: "build a package or a tree",
	Long:  `build packages or trees from luet tree definitions`,
	Run: func(cmd *cobra.Command, args []string) {

		src := viper.GetString("tree")
		dst := viper.GetString("output")
		concurrency := viper.GetInt("concurrency")
		backendType := viper.GetString("backend")
		privileged := viper.GetBool("privileged")

		if len(args) != 3 {
			Fatal("Incorrect number of arguments")
		}

		category := args[0]
		name := args[1]
		version := args[2]

		var compilerBackend compiler.CompilerBackend

		switch backendType {
		case "img":
			compilerBackend = backend.NewSimpleImgBackend()
		case "docker":
			compilerBackend = backend.NewSimpleDockerBackend()
		}

		generalRecipe := tree.NewCompilerRecipe()

		Info("Loading", src)
		err := generalRecipe.Load(src)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
		compiler := compiler.NewLuetCompiler(compilerBackend, generalRecipe.Tree())
		spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: name, Category: category, Version: version})
		if err != nil {
			Fatal("Error: " + err.Error())
		}

		spec.SetOutputPath(dst)
		artifact, err := compiler.Compile(concurrency, privileged, spec)
		if err != nil {
			Fatal("Error: " + err.Error())
		}

		Info("Artifact generated:", artifact.GetPath())
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	buildCmd.Flags().String("tree", path, "Source luet tree")
	viper.BindPFlag("tree", buildCmd.Flags().Lookup("tree"))
	buildCmd.Flags().String("output", path, "Destination folder")
	viper.BindPFlag("output", buildCmd.Flags().Lookup("output"))
	buildCmd.Flags().String("backend", "docker", "backend used (docker,img)")
	viper.BindPFlag("backend", buildCmd.Flags().Lookup("backend"))
	buildCmd.Flags().Int("concurrency", runtime.NumCPU(), "Concurrency")
	viper.BindPFlag("concurrency", buildCmd.Flags().Lookup("concurrency"))
	buildCmd.Flags().Bool("privileged", false, "Privileged (Keep permissions)")
	viper.BindPFlag("privileged", buildCmd.Flags().Lookup("privileged"))
	RootCmd.AddCommand(buildCmd)
}
