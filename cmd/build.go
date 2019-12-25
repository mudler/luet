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
	"io/ioutil"
	"os"
	"runtime"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var buildCmd = &cobra.Command{
	Use:   "build <package name> <package name> <package name> ...",
	Short: "build a package or a tree",
	Long:  `build packages or trees from luet tree definitions. Packages are in [category]/[name]-[version] form`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("tree", cmd.Flags().Lookup("tree"))
		viper.BindPFlag("destination", cmd.Flags().Lookup("destination"))
		viper.BindPFlag("backend", cmd.Flags().Lookup("backend"))
		viper.BindPFlag("concurrency", cmd.Flags().Lookup("concurrency"))
		viper.BindPFlag("privileged", cmd.Flags().Lookup("privileged"))
		viper.BindPFlag("database", cmd.Flags().Lookup("database"))
		viper.BindPFlag("revdeps", cmd.Flags().Lookup("revdeps"))
		viper.BindPFlag("all", cmd.Flags().Lookup("all"))
		viper.BindPFlag("compression", cmd.Flags().Lookup("compression"))
	},
	Run: func(cmd *cobra.Command, args []string) {

		src := viper.GetString("tree")
		dst := viper.GetString("destination")
		concurrency := viper.GetInt("concurrency")
		backendType := viper.GetString("backend")
		privileged := viper.GetBool("privileged")
		revdeps := viper.GetBool("revdeps")
		all := viper.GetBool("all")
		databaseType := viper.GetString("database")
		compressionType := viper.GetString("compression")

		compilerSpecs := compiler.NewLuetCompilationspecs()
		var compilerBackend compiler.CompilerBackend
		var db pkg.PackageDatabase
		switch backendType {
		case "img":
			compilerBackend = backend.NewSimpleImgBackend()
		case "docker":
			compilerBackend = backend.NewSimpleDockerBackend()
		}

		switch databaseType {
		case "memory":
			db = pkg.NewInMemoryDatabase(false)

		case "boltdb":
			tmpdir, err := ioutil.TempDir("", "package")
			if err != nil {
				Fatal(err)
			}
			db = pkg.NewBoltDatabase(tmpdir)

		}
		defer db.Clean()

		generalRecipe := tree.NewCompilerRecipe(db)

		Info("Loading", src)
		Info("Building in", dst)

		err := generalRecipe.Load(src)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
		luetCompiler := compiler.NewLuetCompiler(compilerBackend, generalRecipe.GetDatabase())
		luetCompiler.SetConcurrency(concurrency)
		luetCompiler.SetCompressionType(compiler.CompressionImplementation(compressionType))
		if !all {
			for _, a := range args {
				gp, err := _gentoo.ParsePackageStr(a)
				if err != nil {
					Fatal("Invalid package string ", a, ": ", err.Error())
				}

				pack := &pkg.DefaultPackage{
					Name:     gp.Name,
					Version:  fmt.Sprintf("%s%s%s", gp.Condition.String(), gp.Version, gp.VersionSuffix),
					Category: gp.Category,
					Uri:      make([]string, 0),
				}
				spec, err := luetCompiler.FromPackage(pack)
				if err != nil {
					Fatal("Error: " + err.Error())
				}

				spec.SetOutputPath(dst)
				compilerSpecs.Add(spec)
			}
		} else {
			w := generalRecipe.GetDatabase().World()

			for _, p := range w {
				spec, err := luetCompiler.FromPackage(p)
				if err != nil {
					Fatal("Error: " + err.Error())
				}
				Info(":package: Selecting ", p.GetName(), p.GetVersion())
				spec.SetOutputPath(dst)
				compilerSpecs.Add(spec)
			}
		}

		var artifact []compiler.Artifact
		var errs []error
		if revdeps {
			artifact, errs = luetCompiler.CompileWithReverseDeps(privileged, compilerSpecs)

		} else {
			artifact, errs = luetCompiler.CompileParallel(privileged, compilerSpecs)

		}
		if len(errs) != 0 {
			for _, e := range errs {
				Error("Error: " + e.Error())
			}
			Fatal("Bailing out")
		}
		for _, a := range artifact {
			Info("Artifact generated:", a.GetPath())
		}
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	buildCmd.Flags().String("tree", path, "Source luet tree")
	buildCmd.Flags().String("backend", "docker", "backend used (docker,img)")
	buildCmd.Flags().Int("concurrency", runtime.NumCPU(), "Concurrency")
	buildCmd.Flags().Bool("privileged", false, "Privileged (Keep permissions)")
	buildCmd.Flags().String("database", "memory", "database used for solving (memory,boltdb)")
	buildCmd.Flags().Bool("revdeps", false, "Build with revdeps")
	buildCmd.Flags().Bool("all", false, "Build all packages in the tree")
	buildCmd.Flags().String("destination", path, "Destination folder")
	buildCmd.Flags().String("compression", "none", "Compression alg: none, gzip")

	RootCmd.AddCommand(buildCmd)
}
