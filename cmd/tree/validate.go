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

func validateWorker(i int,
	wg *sync.WaitGroup,
	c <-chan pkg.Package,
	reciper tree.Builder,
	withSolver bool,
	regExcludes, regMatches []*regexp.Regexp,
	excludes, matches []string,
	errs chan error) {

	defer wg.Done()

	var depSolver solver.PackageSolver
	var cacheDeps *pkg.InMemoryDatabase
	brokenPkgs := 0
	brokenDeps := 0
	var errstr string

	emptyInstallationDb := pkg.NewInMemoryDatabase(false)
	if withSolver {
		depSolver = solver.NewSolver(pkg.NewInMemoryDatabase(false),
			reciper.GetDatabase(),
			emptyInstallationDb)

		// Use Singleton in memory cache for speedup dependencies
		// analysis
		cacheDeps = pkg.NewInMemoryDatabase(true).(*pkg.InMemoryDatabase)
	}

	for p := range c {

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
			Error(fmt.Sprintf("%s/%s-%s: Broken. No versions could be found by database %s",
				p.GetCategory(), p.GetName(), p.GetVersion(),
				errstr,
			))

			errs <- errors.New(
				fmt.Sprintf("%s/%s-%s: Broken. No versions could be found by database %s",
					p.GetCategory(), p.GetName(), p.GetVersion(),
					errstr,
				))

			brokenPkgs++
		}

		pkgstr := fmt.Sprintf("%s/%s-%s", p.GetCategory(), p.GetName(),
			p.GetVersion())

		validpkg := true

		if len(matches) > 0 {
			matched := false
			for _, rgx := range regMatches {
				if rgx.MatchString(pkgstr) {
					matched = true
					break
				}
			}

			if !matched {
				continue
			}
		}

		if len(excludes) > 0 {
			excluded := false
			for _, rgx := range regExcludes {
				if rgx.MatchString(pkgstr) {
					excluded = true
					break
				}
			}

			if excluded {
				continue
			}
		}

		Info("Checking package "+
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
				Error(fmt.Sprintf("%s/%s-%s: Broken Dep %s/%s-%s - %s",
					p.GetCategory(), p.GetName(), p.GetVersion(),
					r.GetCategory(), r.GetName(), r.GetVersion(),
					errstr,
				))

				errs <- errors.New(
					fmt.Sprintf("%s/%s-%s: Broken Dep %s/%s-%s - %s",
						p.GetCategory(), p.GetName(), p.GetVersion(),
						r.GetCategory(), r.GetName(), r.GetVersion(),
						errstr))

				brokenDeps++

				validpkg = false

			} else {

				Debug("Find packages for dep",
					fmt.Sprintf("%s/%s-%s", r.GetCategory(), r.GetName(), r.GetVersion()))

				if withSolver {

					Info(fmt.Sprintf("  :soap: [%2d/%2d] %s/%s-%s: %s/%s-%s",
						idx+1, len(all),
						p.GetCategory(), p.GetName(), p.GetVersion(),
						r.GetCategory(), r.GetName(), r.GetVersion(),
					))

					// Check if the solver is already been done for the deep
					_, err := cacheDeps.Get(r.HashFingerprint())
					if err == nil {
						Debug("  :direct_hit: Cache Hit for dep",
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

						Error(fmt.Sprintf("%s/%s-%s: solver broken for dep %s/%s-%s - %s",
							p.GetCategory(), p.GetName(), p.GetVersion(),
							r.GetCategory(), r.GetName(), r.GetVersion(),
							err.Error(),
						))

						errs <- errors.New(
							fmt.Sprintf("%s/%s-%s: solver broken for Dep %s/%s-%s - %s",
								p.GetCategory(), p.GetName(), p.GetVersion(),
								r.GetCategory(), r.GetName(), r.GetVersion(),
								err.Error()))

						brokenDeps++
						validpkg = false
					}

					// Register the key
					cacheDeps.Set(r.HashFingerprint(), "1")

				}
			}

		}

		if !validpkg {
			brokenPkgs++
		}
	}
}

func NewTreeValidateCommand() *cobra.Command {
	var excludes []string
	var matches []string
	var treePaths []string

	var ans = &cobra.Command{
		Use:   "validate [OPTIONS]",
		Short: "Validate a tree or a list of packages",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(treePaths) < 1 {
				Fatal("Mandatory tree param missing.")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			concurrency := LuetCfg.GetGeneral().Concurrency

			withSolver, _ := cmd.Flags().GetBool("with-solver")

			reciper := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
			for _, treePath := range treePaths {
				err := reciper.Load(treePath)
				if err != nil {
					Fatal("Error on load tree ", err)
				}
			}

			regExcludes, err := helpers.CreateRegexArray(excludes)
			if err != nil {
				Fatal(err.Error())
			}
			regMatches, err := helpers.CreateRegexArray(matches)
			if err != nil {
				Fatal(err.Error())
			}

			all := make(chan pkg.Package)
			errs := make(chan error)

			var wg = new(sync.WaitGroup)

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go validateWorker(i, wg, all,
					reciper, withSolver, regExcludes, regMatches, excludes, matches,
					errs)
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
				Fatal("Errors: " + strconv.Itoa(len(stringerrs)))
				//	if brokenPkgs > 0 {
				//os.Exit(1)
			} else {
				Info("All good! :white_check_mark:")
				os.Exit(0)
			}
		},
	}

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
