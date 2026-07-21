// Copyright © 2019-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package solver_test

import (
	"fmt"
	"strings"
	"testing"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"

	. "github.com/mudler/luet/pkg/solver"
)

// This file provides a repository-shaped fixture for benchmarking, and standard
// testing.B benchmarks built on it.
//
// It exists because the pre-existing Ginkgo benchmark suites do not run at all:
// they are written against Ginkgo v1's Measure(), which in v2 is a deprecated
// stub that ignores its arguments and returns true, so the closures are never
// executed. They also build worlds of uniquely-named, dependency-free packages,
// which exercise none of the multi-version resolution paths that dominate a
// real upgrade.
//
// The generator below deliberately produces the shape a distribution actually
// has: a small core of dependency-free packages, progressively wider layers
// above it, several versions per package, and dependencies expressed as version
// selectors rather than exact pins.
//
// It deliberately does NOT generate conflicts. Every package here is installed
// and upgradable, so a conflict between two of them makes the world
// unsatisfiable and the benchmark measures how fast the solver reaches UNSAT
// rather than how fast it resolves. Benchmarking the conflict path needs a
// world where conflicting packages are not all installed at once; that is worth
// adding, but it is a different fixture.

// lcg is a tiny deterministic pseudo-random source. Benchmarks must be
// reproducible run-to-run, so we cannot use math/rand's global source.
type lcg uint64

func (r *lcg) next(n int) int {
	*r = lcg(uint64(*r)*6364136223846793005 + 1442695040888963407)
	return int((*r >> 33)) % n
}

// worldOpts describes the shape of a generated package world.
type worldOpts struct {
	// Packages is the total number of package families (each with several
	// versions).
	Packages int
	// Versions is how many versions each family has. The oldest is the one
	// recorded as installed, so every family is upgradable.
	Versions int
	// TightRatio is the percentage of dependencies that demand a minimum
	// version above the oldest (">=2.0") rather than accepting anything
	// (">=1.0"). Both forms leave the solver a choice between versions, which
	// is what exercises the version-selection path; the tight form additionally
	// rules the installed version out.
	TightRatio int
}

// defaultWorld keeps TightRatio at 0, i.e. every dependency accepts any
// version.
//
// Anything above 0 makes Solver.Upgrade fail outright with "couldn't uninstall
// candidates ... could not satisfy the constraints". The reason is structural:
// Uninstall builds its sub-solver over installedcopy, which holds only the
// versions currently installed (here, the oldest of each). A ">=2.0" dependency
// has no candidate in that restricted world, so its at-least-one clause is
// emitted empty and the formula is unsatisfiable.
//
// That is worth a closer look on its own - a system whose packages declare a
// minimum version above what is installed appears to be unupgradable - but it
// is a correctness question, not a benchmarking one, so the default world
// avoids it.
func defaultWorld(n int) worldOpts {
	return worldOpts{Packages: n, Versions: 3, TightRatio: 0}
}

// layerOf assigns a package to a dependency layer, approximating a real
// distribution: a narrow base of core packages, widening towards leaf
// applications. Dependencies only ever point at strictly lower layers, which
// keeps the graph acyclic without needing a cycle check.
func layerOf(i, total int) int {
	switch pos := float64(i) / float64(total); {
	case pos < 0.05:
		return 0 // core: no dependencies
	case pos < 0.30:
		return 1 // libraries
	case pos < 0.70:
		return 2 // middleware
	default:
		return 3 // applications (leaves)
	}
}

// buildWorld returns (definitions, installed, solverdb).
//
// definitions holds every version of every package; installed holds only the
// oldest version of each, so the whole system is upgradable. Returns the
// generated definition set so callers can pick targets.
func buildWorld(o worldOpts) (types.PackageDatabase, types.PackageDatabase, types.PackageDatabase) {
	defs := pkg.NewInMemoryDatabase(false)
	installed := pkg.NewInMemoryDatabase(false)
	solverdb := pkg.NewInMemoryDatabase(false)

	rnd := lcg(42)

	// Index of package families by layer, so dependencies can be drawn from
	// strictly lower layers only.
	byLayer := map[int][]int{}

	for i := 0; i < o.Packages; i++ {
		layer := layerOf(i, o.Packages)
		byLayer[layer] = append(byLayer[layer], i)

		// Draw dependencies from lower layers.
		var candidates []int
		for l := 0; l < layer; l++ {
			candidates = append(candidates, byLayer[l]...)
		}

		nDeps := 0
		if len(candidates) > 0 {
			// Core has none; upper layers take between 1 and 4.
			nDeps = 1 + rnd.next(4)
			if nDeps > len(candidates) {
				nDeps = len(candidates)
			}
		}

		seen := map[int]bool{}
		var requires []*types.Package
		for d := 0; d < nDeps; d++ {
			target := candidates[rnd.next(len(candidates))]
			if seen[target] {
				continue
			}
			seen[target] = true

			// Both forms are selectors, so the world stays satisfiable when
			// every package moves to its newest version. Pinning an exact old
			// version here would instead make the world unsat the moment its
			// dependents upgrade, which measures nothing.
			constraint := ">=1.0"
			if rnd.next(100) < o.TightRatio {
				constraint = ">=2.0"
			}
			requires = append(requires,
				types.NewPackage(fmt.Sprintf("pkg%d", target), constraint, nil, nil))
		}

		for v := 0; v < o.Versions; v++ {
			version := fmt.Sprintf("%d.0", v+1)
			p := types.NewPackage(fmt.Sprintf("pkg%d", i), version, requires, nil)
			if _, err := defs.CreatePackage(p); err != nil {
				panic(err)
			}
			// Oldest version is what the system currently has.
			if v == 0 {
				if _, err := installed.CreatePackage(p); err != nil {
					panic(err)
				}
			}
		}
	}

	return defs, installed, solverdb
}

func newBenchSolver(defs, installed, solverdb types.PackageDatabase) types.PackageSolver {
	return NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		installed, defs, solverdb)
}

// benchmarkUpgrade measures Solver.Upgrade for a given checkconflicts value.
//
// checkconflicts is what `luet upgrade --full` controls: installer.computeUpgrade
// calls Upgrade(l.Options.FullUninstall, true), so --full sets checkconflicts.
// false is the default `luet upgrade` path, true is `luet upgrade --full`.
func benchmarkUpgrade(b *testing.B, n int, checkconflicts bool) {
	o := defaultWorld(n)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		defs, installed, solverdb := buildWorld(o)
		s := newBenchSolver(defs, installed, solverdb)
		b.StartTimer()

		_, _, err := s.Upgrade(checkconflicts, true)
		b.StopTimer()
		if err != nil {
			// checkconflicts=true currently cannot complete on any world where
			// an upgradable package has reverse dependencies - see
			// TestUpgradeCheckConflictsFailsOnRevdeps below. Skip rather than
			// fail, so these benchmarks begin reporting numbers as soon as that
			// is fixed instead of having to be written from scratch.
			b.Skipf("upgrade failed (checkconflicts=%v): %s", checkconflicts, err)
		}
		b.StartTimer()
	}
}

// Default `luet upgrade`.
func BenchmarkUpgradeDefault200(b *testing.B) { benchmarkUpgrade(b, 200, false) }
func BenchmarkUpgradeDefault400(b *testing.B) { benchmarkUpgrade(b, 400, false) }
func BenchmarkUpgradeDefault800(b *testing.B) { benchmarkUpgrade(b, 800, false) }

// `luet upgrade --full`.
func BenchmarkUpgradeFull200(b *testing.B) { benchmarkUpgrade(b, 200, true) }
func BenchmarkUpgradeFull400(b *testing.B) { benchmarkUpgrade(b, 400, true) }
func BenchmarkUpgradeFull800(b *testing.B) { benchmarkUpgrade(b, 800, true) }

// BenchmarkWorldBuild isolates fixture construction, so the numbers above can
// be read net of the database population cost.
func BenchmarkWorldBuild400(b *testing.B) {
	o := defaultWorld(400)
	for i := 0; i < b.N; i++ {
		buildWorld(o)
	}
}

// TestUpgradeCheckConflictsFailsOnRevdeps documents current behaviour: an
// upgrade run with checkconflicts=true aborts on any world where a package
// being replaced has reverse dependencies - even when no package declares a
// Conflicts entry at all.
//
// The cause is that Solver.Conflicts does not examine declared conflicts. It
// collects the candidate's reverse dependencies and returns true if there are
// any, building an error that lists them. Uninstall's early-return branch turns
// that into a hard failure for the whole upgrade.
//
// This matters because installer.computeUpgrade calls
// Upgrade(l.Options.FullUninstall, true), so checkconflicts is exactly what
// `luet upgrade --full` sets - meaning `--full` cannot succeed on a realistic
// system. It was also the default until commit 59d78c3f (Dec 2020) dropped the
// negation on that argument.
//
// If this test starts failing, the underlying behaviour changed and the
// benchmarks above should start producing numbers.
func TestUpgradeCheckConflictsFailsOnRevdeps(t *testing.T) {
	// A world with no declared conflicts whatsoever.
	defs, installed, solverdb := buildWorld(defaultWorld(50))
	s := newBenchSolver(defs, installed, solverdb)

	if _, _, err := s.Upgrade(false, true); err != nil {
		t.Fatalf("checkconflicts=false should succeed, got: %s", err)
	}

	defs, installed, solverdb = buildWorld(defaultWorld(50))
	s = newBenchSolver(defs, installed, solverdb)

	_, _, err := s.Upgrade(true, true)
	if err == nil {
		t.Skip("checkconflicts=true now succeeds - the revdeps-as-conflicts " +
			"behaviour appears fixed; re-enable the Full benchmarks")
	}
	if !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected a conflicts-related failure, got: %s", err)
	}
}
