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
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/types/artifact"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	"github.com/mudler/luet/pkg/installer"

	"github.com/mudler/luet/pkg/compiler/types/compression"
	"github.com/mudler/luet/pkg/compiler/types/options"
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build <package name> <package name> <package name> ...",
	Short: "build a package or a tree",
	Long: `Builds one or more packages from a tree (current directory is implied):

	$ luet build utils/busybox utils/yq ...

Builds all packages

	$ luet build --all

Builds only the leaf packages:

	$ luet build --full

Build package revdeps:

	$ luet build --revdeps utils/yq

Build package without dependencies (needs the images already in the host, or either need to be available online):

	$ luet build --nodeps utils/yq ...

Build packages specifying multiple definition trees:

	$ luet build --tree overlay/path --tree overlay/path2 utils/yq ...
`, PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("tree", cmd.Flags().Lookup("tree"))
		LuetCfg.Viper.BindPFlag("destination", cmd.Flags().Lookup("destination"))
		LuetCfg.Viper.BindPFlag("backend", cmd.Flags().Lookup("backend"))
		LuetCfg.Viper.BindPFlag("privileged", cmd.Flags().Lookup("privileged"))
		LuetCfg.Viper.BindPFlag("revdeps", cmd.Flags().Lookup("revdeps"))
		LuetCfg.Viper.BindPFlag("all", cmd.Flags().Lookup("all"))
		LuetCfg.Viper.BindPFlag("compression", cmd.Flags().Lookup("compression"))
		LuetCfg.Viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		LuetCfg.Viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		util.BindValuesFlags(cmd)
		LuetCfg.Viper.BindPFlag("backend-args", cmd.Flags().Lookup("backend-args"))

		LuetCfg.Viper.BindPFlag("image-repository", cmd.Flags().Lookup("image-repository"))
		LuetCfg.Viper.BindPFlag("push", cmd.Flags().Lookup("push"))
		LuetCfg.Viper.BindPFlag("pull", cmd.Flags().Lookup("pull"))
		LuetCfg.Viper.BindPFlag("wait", cmd.Flags().Lookup("wait"))
		LuetCfg.Viper.BindPFlag("keep-images", cmd.Flags().Lookup("keep-images"))

		util.BindSolverFlags(cmd)

		LuetCfg.Viper.BindPFlag("general.show_build_output", cmd.Flags().Lookup("live-output"))
		LuetCfg.Viper.BindPFlag("backend-args", cmd.Flags().Lookup("backend-args"))

	},
	Run: func(cmd *cobra.Command, args []string) {

		treePaths := LuetCfg.Viper.GetStringSlice("tree")
		dst := LuetCfg.Viper.GetString("destination")
		concurrency := LuetCfg.GetGeneral().Concurrency
		backendType := LuetCfg.Viper.GetString("backend")
		privileged := LuetCfg.Viper.GetBool("privileged")
		revdeps := LuetCfg.Viper.GetBool("revdeps")
		all := LuetCfg.Viper.GetBool("all")
		compressionType := LuetCfg.Viper.GetString("compression")
		imageRepository := LuetCfg.Viper.GetString("image-repository")
		values := util.ValuesFlags()
		wait := LuetCfg.Viper.GetBool("wait")
		push := LuetCfg.Viper.GetBool("push")
		pull := LuetCfg.Viper.GetBool("pull")
		keepImages := LuetCfg.Viper.GetBool("keep-images")
		nodeps := LuetCfg.Viper.GetBool("nodeps")
		onlydeps := LuetCfg.Viper.GetBool("onlydeps")
		onlyTarget, _ := cmd.Flags().GetBool("only-target-package")
		full, _ := cmd.Flags().GetBool("full")
		rebuild, _ := cmd.Flags().GetBool("rebuild")

		var results Results
		backendArgs := LuetCfg.Viper.GetStringSlice("backend-args")

		out, _ := cmd.Flags().GetString("output")
		if out != "terminal" {
			LuetCfg.GetLogging().SetLogLevel("error")
		}
		pretend, _ := cmd.Flags().GetBool("pretend")
		fromRepo, _ := cmd.Flags().GetBool("from-repositories")

		compilerSpecs := compilerspec.NewLuetCompilationspecs()
		var db pkg.PackageDatabase

		compilerBackend, err := compiler.NewBackend(backendType)
		helpers.CheckErr(err)

		db = pkg.NewInMemoryDatabase(false)
		defer db.Clean()

		generalRecipe := tree.NewCompilerRecipe(db)

		if fromRepo {
			if err := installer.LoadBuildTree(generalRecipe, db, LuetCfg); err != nil {
				Warning("errors while loading trees from repositories", err.Error())
			}
		}

		for _, src := range treePaths {
			Info("Loading tree", src)
			helpers.CheckErr(generalRecipe.Load(src))
		}

		Info("Building in", dst)

		opts := util.SetSolverConfig()
		pullRepo, _ := cmd.Flags().GetStringArray("pull-repository")

		LuetCfg.GetGeneral().ShowBuildOutput = LuetCfg.Viper.GetBool("general.show_build_output")

		Debug("Solver", opts.CompactString())

		opts.Options = solver.Options{Type: solver.SingleCoreSimple, Concurrency: concurrency}

		luetCompiler := compiler.NewLuetCompiler(compilerBackend, generalRecipe.GetDatabase(),
			options.NoDeps(nodeps),
			options.WithBackendType(backendType),
			options.PushImages(push),
			options.WithBuildValues(values),
			options.WithPullRepositories(pullRepo),
			options.WithPushRepository(imageRepository),
			options.Rebuild(rebuild),
			options.WithTemplateFolder(util.TemplateFolders(fromRepo, treePaths)),
			options.WithSolverOptions(*opts),
			options.Wait(wait),
			options.OnlyTarget(onlyTarget),
			options.PullFirst(pull),
			options.KeepImg(keepImages),
			options.OnlyDeps(onlydeps),
			options.BackendArgs(backendArgs),
			options.Concurrency(concurrency),
			options.WithCompressionType(compression.Implementation(compressionType)),
		)

		if full {
			specs, err := luetCompiler.FromDatabase(generalRecipe.GetDatabase(), true, dst)
			if err != nil {
				Fatal(err.Error())
			}
			for _, spec := range specs {
				Info(":package: Selecting ", spec.GetPackage().GetName(), spec.GetPackage().GetVersion())

				compilerSpecs.Add(spec)
			}
		} else if !all {
			for _, a := range args {
				pack, err := helpers.ParsePackageStr(a)
				if err != nil {
					Fatal("Invalid package string ", a, ": ", err.Error())
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

		var artifact []*artifact.PackageArtifact
		var errs []error
		if revdeps {
			artifact, errs = luetCompiler.CompileWithReverseDeps(privileged, compilerSpecs)

		} else if pretend {
			toCalculate := []*compilerspec.LuetCompilationSpec{}
			if full {
				var err error
				toCalculate, err = luetCompiler.ComputeMinimumCompilableSet(compilerSpecs.All()...)
				if err != nil {
					errs = append(errs, err)
				}
			} else {
				toCalculate = compilerSpecs.All()
			}

			for _, sp := range toCalculate {
				ht := compiler.NewHashTree(generalRecipe.GetDatabase())
				hashTree, err := ht.Query(luetCompiler, sp)
				if err != nil {
					errs = append(errs, err)
				}
				for _, p := range hashTree.Dependencies {
					results.Packages = append(results.Packages,
						PackageResult{
							Name:       p.Package.GetName(),
							Version:    p.Package.GetVersion(),
							Category:   p.Package.GetCategory(),
							Repository: "",
							Hidden:     p.Package.IsHidden(),
							Target:     sp.GetPackage().HumanReadableString(),
						})
				}
			}

			y, err := yaml.Marshal(results)
			if err != nil {
				fmt.Printf("err: %v\n", err)
				return
			}
			switch out {
			case "yaml":
				fmt.Println(string(y))
			case "json":
				j2, err := yaml.YAMLToJSON(y)
				if err != nil {
					fmt.Printf("err: %v\n", err)
					return
				}
				fmt.Println(string(j2))
			case "terminal":
				for _, p := range results.Packages {
					Info(p.String())
				}
			}
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
			Info("Artifact generated:", a.Path)
		}
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}

	buildCmd.Flags().StringSliceP("tree", "t", []string{path}, "Path of the tree to use.")
	buildCmd.Flags().String("backend", "docker", "backend used (docker,img)")
	buildCmd.Flags().Bool("privileged", true, "Privileged (Keep permissions)")
	buildCmd.Flags().Bool("revdeps", false, "Build with revdeps")
	buildCmd.Flags().Bool("all", false, "Build all specfiles in the tree")
	buildCmd.Flags().Bool("full", false, "Build all packages (optimized)")
	buildCmd.Flags().StringSlice("values", []string{}, "Build values file to interpolate with each package")
	buildCmd.Flags().StringSliceP("backend-args", "a", []string{}, "Backend args")

	buildCmd.Flags().String("destination", filepath.Join(path, "build"), "Destination folder")
	buildCmd.Flags().String("compression", "none", "Compression alg: none, gzip, zstd")
	buildCmd.Flags().String("image-repository", "luet/cache", "Default base image string for generated image")
	buildCmd.Flags().Bool("push", false, "Push images to a hub")
	buildCmd.Flags().Bool("pull", false, "Pull images from a hub")
	buildCmd.Flags().Bool("wait", false, "Don't build all intermediate images, but wait for them until they are available")
	buildCmd.Flags().Bool("keep-images", true, "Keep built docker images in the host")
	buildCmd.Flags().Bool("nodeps", false, "Build only the target packages, skipping deps (it works only if you already built the deps locally, or by using --pull) ")
	buildCmd.Flags().Bool("onlydeps", false, "Build only package dependencies")
	buildCmd.Flags().Bool("only-target-package", false, "Build packages of only the required target. Otherwise builds all the necessary ones not present in the destination")
	buildCmd.Flags().String("solver-type", "", "Solver strategy")
	buildCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	buildCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	buildCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	buildCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	buildCmd.Flags().Bool("live-output", LuetCfg.GetGeneral().ShowBuildOutput, "Enable live output of the build phase.")
	buildCmd.Flags().Bool("from-repositories", false, "Consume the user-defined repositories to pull specfiles from")
	buildCmd.Flags().Bool("rebuild", false, "To combine with --pull. Allows to rebuild the target package even if an image is available, against a local values file")
	buildCmd.Flags().Bool("pretend", false, "Just print what packages will be compiled")
	buildCmd.Flags().StringArrayP("pull-repository", "p", []string{}, "A list of repositories to pull the cache from")

	buildCmd.Flags().StringP("output", "o", "terminal", "Output format ( Defaults: terminal, available: json,yaml )")

	RootCmd.AddCommand(buildCmd)
}
