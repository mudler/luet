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
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"

	helpers "github.com/mudler/luet/cmd/helpers"
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/spf13/cobra"
)

type ValidateOpts struct {
	WithSolver    bool
	OnlyRuntime   bool
	OnlyBuildtime bool
	RegExcludes   []*regexp.Regexp
	RegMatches    []*regexp.Regexp
	Excludes      []string
	Matches       []string

	// Runtime validate stuff
	RuntimeCacheDeps *pkg.InMemoryDatabase
	RuntimeReciper   *tree.InstallerRecipe

	// Buildtime validate stuff
	BuildtimeCacheDeps *pkg.InMemoryDatabase
	BuildtimeReciper   *tree.CompilerRecipe

	Mutex      sync.Mutex
	BrokenPkgs int
	BrokenDeps int
}

func (o *ValidateOpts) IncrBrokenPkgs() {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.BrokenPkgs++
}

func (o *ValidateOpts) IncrBrokenDeps() {
	o.Mutex.Lock()
	defer o.Mutex.Unlock()
	o.BrokenDeps++
}

func validatePackage(p pkg.Package, checkType string, opts *ValidateOpts, reciper tree.Builder, cacheDeps *pkg.InMemoryDatabase) error {
	var errstr string
	var ans error

	var depSolver solver.PackageSolver

	if opts.WithSolver {
		emptyInstallationDb := pkg.NewInMemoryDatabase(false)
		depSolver = solver.NewSolver(pkg.NewInMemoryDatabase(false),
			reciper.GetDatabase(),
			emptyInstallationDb)
	}

	found, err := reciper.GetDatabase().FindPackages(
		&pkg.DefaultPackage{
			Name:     p.GetName(),
			Category: p.GetCategory(),
			Version:  ">=0",
		},
	)

	if err != nil || len(found) < 1 {
		if err != nil {
			errstr = err.Error()
		} else {
			errstr = "No packages"
		}
		Error(fmt.Sprintf("[%9s] %s/%s-%s: Broken. No versions could be found by database %s",
			checkType,
			p.GetCategory(), p.GetName(), p.GetVersion(),
			errstr,
		))

		opts.IncrBrokenDeps()

		return errors.New(
			fmt.Sprintf("[%9s] %s/%s-%s: Broken. No versions could be found by database %s",
				checkType,
				p.GetCategory(), p.GetName(), p.GetVersion(),
				errstr,
			))
	}

	// Ensure that we use the right package from right recipier for deps
	pReciper, err := reciper.GetDatabase().FindPackage(
		&pkg.DefaultPackage{
			Name:     p.GetName(),
			Category: p.GetCategory(),
			Version:  p.GetVersion(),
		},
	)
	if err != nil {
		errstr = fmt.Sprintf("[%9s] %s/%s-%s: Error on retrieve package - %s.",
			checkType,
			p.GetCategory(), p.GetName(), p.GetVersion(),
			err.Error(),
		)
		Error(errstr)

		return errors.New(errstr)
	}
	p = pReciper

	pkgstr := fmt.Sprintf("%s/%s-%s", p.GetCategory(), p.GetName(),
		p.GetVersion())

	validpkg := true

	if len(opts.Matches) > 0 {
		matched := false
		for _, rgx := range opts.RegMatches {
			if rgx.MatchString(pkgstr) {
				matched = true
				break
			}
		}

		if !matched {
			return nil
		}
	}

	if len(opts.Excludes) > 0 {
		excluded := false
		for _, rgx := range opts.RegExcludes {
			if rgx.MatchString(pkgstr) {
				excluded = true
				break
			}
		}

		if excluded {
			return nil
		}
	}

	Info(fmt.Sprintf("[%9s] Checking package ", checkType)+
		fmt.Sprintf("%s/%s-%s", p.GetCategory(), p.GetName(), p.GetVersion()),
		"with", len(p.GetRequires()), "dependencies and", len(p.GetConflicts()), "conflicts.")

	all := p.GetRequires()
	all = append(all, p.GetConflicts()...)
	for idx, r := range all {

		var deps pkg.Packages
		var err error
		if r.IsSelector() {
			deps, err = reciper.GetDatabase().FindPackages(
				&pkg.DefaultPackage{
					Name:     r.GetName(),
					Category: r.GetCategory(),
					Version:  r.GetVersion(),
				},
			)
		} else {
			deps = append(deps, r)
		}

		if err != nil || len(deps) < 1 {
			if err != nil {
				errstr = err.Error()
			} else {
				errstr = "No packages"
			}
			Error(fmt.Sprintf("[%9s] %s/%s-%s: Broken Dep %s/%s-%s - %s",
				checkType,
				p.GetCategory(), p.GetName(), p.GetVersion(),
				r.GetCategory(), r.GetName(), r.GetVersion(),
				errstr,
			))

			opts.IncrBrokenDeps()

			ans = errors.New(
				fmt.Sprintf("[%9s] %s/%s-%s: Broken Dep %s/%s-%s - %s",
					checkType,
					p.GetCategory(), p.GetName(), p.GetVersion(),
					r.GetCategory(), r.GetName(), r.GetVersion(),
					errstr))

			validpkg = false

		} else {

			Debug(fmt.Sprintf("[%9s] Find packages for dep", checkType),
				fmt.Sprintf("%s/%s-%s", r.GetCategory(), r.GetName(), r.GetVersion()))

			if opts.WithSolver {

				Info(fmt.Sprintf("[%9s]  :soap: [%2d/%2d] %s/%s-%s: %s/%s-%s",
					checkType,
					idx+1, len(all),
					p.GetCategory(), p.GetName(), p.GetVersion(),
					r.GetCategory(), r.GetName(), r.GetVersion(),
				))

				// Check if the solver is already been done for the deep
				_, err := cacheDeps.Get(r.HashFingerprint(""))
				if err == nil {
					Debug(fmt.Sprintf("[%9s]  :direct_hit: Cache Hit for dep", checkType),
						fmt.Sprintf("%s/%s-%s", r.GetCategory(), r.GetName(), r.GetVersion()))
					continue
				}

				Spinner(32)
				solution, err := depSolver.Install(pkg.Packages{r})
				ass := solution.SearchByName(r.GetPackageName())
				if err == nil {
					_, err = solution.Order(reciper.GetDatabase(), ass.Package.GetFingerPrint())
				}
				SpinnerStop()

				if err != nil {

					Error(fmt.Sprintf("[%9s] %s/%s-%s: solver broken for dep %s/%s-%s - %s",
						checkType,
						p.GetCategory(), p.GetName(), p.GetVersion(),
						r.GetCategory(), r.GetName(), r.GetVersion(),
						err.Error(),
					))

					ans = errors.New(
						fmt.Sprintf("[%9s] %s/%s-%s: solver broken for Dep %s/%s-%s - %s",
							checkType,
							p.GetCategory(), p.GetName(), p.GetVersion(),
							r.GetCategory(), r.GetName(), r.GetVersion(),
							err.Error()))

					opts.IncrBrokenDeps()
					validpkg = false
				}

				// Register the key
				cacheDeps.Set(r.HashFingerprint(""), "1")

			}
		}

	}

	if !validpkg {
		opts.IncrBrokenPkgs()
	}

	return ans
}

func validateWorker(i int,
	wg *sync.WaitGroup,
	c <-chan pkg.Package,
	opts *ValidateOpts,
	errs chan error) {

	defer wg.Done()

	for p := range c {

		if opts.OnlyBuildtime {
			// Check buildtime compiler/deps
			err := validatePackage(p, "buildtime", opts, opts.BuildtimeReciper, opts.BuildtimeCacheDeps)
			if err != nil {
				errs <- err
			}
		} else if opts.OnlyRuntime {

			// Check runtime installer/deps
			err := validatePackage(p, "runtime", opts, opts.RuntimeReciper, opts.RuntimeCacheDeps)
			if err != nil {
				errs <- err
			}

		} else {

			// Check runtime installer/deps
			err := validatePackage(p, "runtime", opts, opts.RuntimeReciper, opts.RuntimeCacheDeps)
			if err != nil {
				errs <- err
				return
			}

			// Check buildtime compiler/deps
			err = validatePackage(p, "buildtime", opts, opts.BuildtimeReciper, opts.BuildtimeCacheDeps)
			if err != nil {
				errs <- err
			}

		}

	}
}

func initOpts(opts *ValidateOpts, onlyRuntime, onlyBuildtime, withSolver bool, treePaths []string) {
	var err error

	opts.OnlyBuildtime = onlyBuildtime
	opts.OnlyRuntime = onlyRuntime
	opts.WithSolver = withSolver
	opts.RuntimeReciper = nil
	opts.BuildtimeReciper = nil
	opts.BrokenPkgs = 0
	opts.BrokenDeps = 0

	if onlyBuildtime {
		opts.BuildtimeReciper = (tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))).(*tree.CompilerRecipe)
	} else if onlyRuntime {
		opts.RuntimeReciper = (tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))).(*tree.InstallerRecipe)
	} else {
		opts.BuildtimeReciper = (tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))).(*tree.CompilerRecipe)
		opts.RuntimeReciper = (tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))).(*tree.InstallerRecipe)
	}

	opts.RuntimeCacheDeps = pkg.NewInMemoryDatabase(false).(*pkg.InMemoryDatabase)
	opts.BuildtimeCacheDeps = pkg.NewInMemoryDatabase(false).(*pkg.InMemoryDatabase)

	for _, treePath := range treePaths {
		Info(fmt.Sprintf("Loading :deciduous_tree: %s...", treePath))
		if opts.BuildtimeReciper != nil {
			err = opts.BuildtimeReciper.Load(treePath)
			if err != nil {
				Fatal("Error on load tree ", err)
			}
		}
		if opts.RuntimeReciper != nil {
			err = opts.RuntimeReciper.Load(treePath)
			if err != nil {
				Fatal("Error on load tree ", err)
			}
		}
	}

	opts.RegExcludes, err = helpers.CreateRegexArray(opts.Excludes)
	if err != nil {
		Fatal(err.Error())
	}
	opts.RegMatches, err = helpers.CreateRegexArray(opts.Matches)
	if err != nil {
		Fatal(err.Error())
	}

}

func NewTreeValidateCommand() *cobra.Command {
	var excludes []string
	var matches []string
	var treePaths []string
	var opts ValidateOpts

	var ans = &cobra.Command{
		Use:   "validate [OPTIONS]",
		Short: "Validate a tree or a list of packages",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			onlyRuntime, _ := cmd.Flags().GetBool("only-runtime")
			onlyBuildtime, _ := cmd.Flags().GetBool("only-buildtime")

			if len(treePaths) < 1 {
				Fatal("Mandatory tree param missing.")
			}
			if onlyRuntime && onlyBuildtime {
				Fatal("Both --only-runtime and --only-buildtime options are not possibile.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			var reciper tree.Builder

			concurrency := LuetCfg.GetGeneral().Concurrency

			withSolver, _ := cmd.Flags().GetBool("with-solver")
			onlyRuntime, _ := cmd.Flags().GetBool("only-runtime")
			onlyBuildtime, _ := cmd.Flags().GetBool("only-buildtime")

			opts.Excludes = excludes
			opts.Matches = matches
			initOpts(&opts, onlyRuntime, onlyBuildtime, withSolver, treePaths)

			// We need at least one valid reciper for get list of the packages.
			if onlyBuildtime {
				reciper = opts.BuildtimeReciper
			} else {
				reciper = opts.RuntimeReciper
			}

			all := make(chan pkg.Package)
			errs := make(chan error)

			var wg = new(sync.WaitGroup)

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go validateWorker(i, wg, all, &opts, errs)
			}
			for _, p := range reciper.GetDatabase().World() {
				all <- p
			}
			close(all)

			// Wait separately and once done close the channel
			go func() {
				wg.Wait()
				close(errs)
			}()

			stringerrs := []string{}
			for e := range errs {
				stringerrs = append(stringerrs, e.Error())
			}
			sort.Strings(stringerrs)
			for _, e := range stringerrs {
				fmt.Println(e)
			}

			// fmt.Println("Broken packages:", brokenPkgs, "(", brokenDeps, "deps ).")
			if len(stringerrs) != 0 {
				Error(fmt.Sprintf("Found %d broken packages and %d broken deps.",
					opts.BrokenPkgs, opts.BrokenDeps))
				Fatal("Errors: " + strconv.Itoa(len(stringerrs)))
			} else {
				Info("All good! :white_check_mark:")
				os.Exit(0)
			}
		},
	}

	ans.Flags().Bool("only-runtime", false, "Check only runtime dependencies.")
	ans.Flags().Bool("only-buildtime", false, "Check only buildtime dependencies.")
	ans.Flags().BoolP("with-solver", "s", false,
		"Enable check of requires also with solver.")
	ans.Flags().StringSliceVarP(&treePaths, "tree", "t", []string{},
		"Path of the tree to use.")
	ans.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{},
		"Exclude matched packages from analysis. (Use string as regex).")
	ans.Flags().StringSliceVarP(&matches, "matches", "m", []string{},
		"Analyze only matched packages. (Use string as regex).")

	return ans
}
