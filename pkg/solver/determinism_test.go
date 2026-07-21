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
	"sort"
	"strings"
	"testing"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"

	. "github.com/mudler/luet/pkg/solver"
)

// Resolution must be DETERMINISTIC and must select the NEWEST SATISFIABLE
// version. Neither property was tested before.
//
// The SAT encoding is version-blind: a package becomes bf.Var("name-cat-ver"),
// an opaque atom, and the at-most-one clauses tying versions together are
// symmetric. Nothing in the formula says one version is newer than another. So
// which version comes back is decided by gophersat's branching, which breaks
// ties on variable index, which is assigned in order of first appearance during
// CNF conversion, which follows the order World() walks the database - a Go map
// range.
//
// The practical consequences, both covered below:
//
//   - the same input can resolve differently run to run
//   - when a conflict blocks the newest version, the solver frequently settles
//     on an older one than it had to
//
// runs is deliberately high enough to catch map-order randomness. Go seeds map
// iteration per range, so a single run proves nothing.
const runs = 60

// installedVersions renders a resolution as a sorted "name-version" list, so
// two runs can be compared as strings.
func installedVersions(asserts types.PackagesAssertions) string {
	var out []string
	for _, a := range asserts {
		if a.Value {
			out = append(out, a.Package.GetName()+"-"+a.Package.GetVersion())
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// conflictWorld builds a world where the newest version of a dependency is
// ruled out by a conflict, so the correct answer is the second-newest.
//
//	base     1.0, 2.0, 3.0
//	mid      1.0, 2.0        requires base >= 1.0
//	app      1.0             requires mid  >= 1.0, conflicts with base 3.0
//
// The only correct resolution installs app-1.0, mid-2.0, base-2.0: base-3.0 is
// forbidden, and base-1.0 is satisfiable but needlessly old.
//
// This shape is why "prefer newest" cannot be a pre-filter. Collapsing the
// candidate set to the newest before solving picks base-3.0, which is UNSAT;
// the alternatives have to survive into the formula for the solver to back off
// to 2.0.
func conflictWorld() (types.PackageDatabase, types.PackageDatabase, types.PackageDatabase) {
	defs := pkg.NewInMemoryDatabase(false)
	installed := pkg.NewInMemoryDatabase(false)
	solverdb := pkg.NewInMemoryDatabase(false)

	for _, v := range []string{"1.0", "2.0", "3.0"} {
		defs.CreatePackage(types.NewPackage("base", v, nil, nil))
	}
	for _, v := range []string{"1.0", "2.0"} {
		defs.CreatePackage(types.NewPackage("mid", v,
			[]*types.Package{types.NewPackage("base", ">=1.0", nil, nil)}, nil))
	}
	defs.CreatePackage(types.NewPackage("app", "1.0",
		[]*types.Package{types.NewPackage("mid", ">=1.0", nil, nil)},
		[]*types.Package{types.NewPackage("base", "3.0", nil, nil)}))

	return defs, installed, solverdb
}

func newSolverFor(defs, installed, solverdb types.PackageDatabase) types.PackageSolver {
	return NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		installed, defs, solverdb)
}

// TestInstallIsDeterministic asserts the same input resolves the same way every
// time. A result that varies run to run is not merely untidy: assertion order
// and content feed SaltedAssertionHash, which keys the compiler's build cache
// and image tags.
func TestInstallIsDeterministic(t *testing.T) {
	seen := map[string]int{}

	for i := 0; i < runs; i++ {
		defs, installed, solverdb := conflictWorld()
		s := newSolverFor(defs, installed, solverdb)

		asserts, err := s.Install(types.Packages{
			types.NewPackage("app", ">=0", nil, nil),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("Install produced %d distinct results over %d runs, want 1:", len(seen), runs)
		for r, n := range seen {
			t.Errorf("  %3d/%d  %s", n, runs, r)
		}
	}
}

// TestInstallSelectsNewestSatisfiable asserts the solver backs off to the
// newest version that actually works, rather than to an arbitrary one.
func TestInstallSelectsNewestSatisfiable(t *testing.T) {
	const want = "app-1.0 base-2.0 mid-2.0"

	seen := map[string]int{}
	for i := 0; i < runs; i++ {
		defs, installed, solverdb := conflictWorld()
		s := newSolverFor(defs, installed, solverdb)

		asserts, err := s.Install(types.Packages{
			types.NewPackage("app", ">=0", nil, nil),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		seen[installedVersions(asserts)]++
	}

	if n := seen[want]; n != runs {
		t.Errorf("selected the newest satisfiable set in %d of %d runs, want %d.\n"+
			"base-3.0 conflicts with app, so base-2.0 is correct; base-1.0 is "+
			"satisfiable but needlessly old.\nobserved:", n, runs, runs)
		for r, c := range seen {
			t.Errorf("  %3d/%d  %s", c, runs, r)
		}
	}
}

// upgradeWorld builds a system where every package has a newer version
// available and packages depend on each other, so an upgrade has to move a
// whole revdep chain at once.
//
//	leaf   1.0 installed, 2.0 and 3.0 available
//	mid    1.0 installed, 2.0 available       requires leaf >= 1.0
//	top    1.0 installed, 2.0 available       requires mid  >= 1.0
func upgradeWorld() (types.PackageDatabase, types.PackageDatabase, types.PackageDatabase) {
	defs := pkg.NewInMemoryDatabase(false)
	installed := pkg.NewInMemoryDatabase(false)
	solverdb := pkg.NewInMemoryDatabase(false)

	leafReq := []*types.Package{types.NewPackage("leaf", ">=1.0", nil, nil)}
	midReq := []*types.Package{types.NewPackage("mid", ">=1.0", nil, nil)}

	for _, v := range []string{"1.0", "2.0", "3.0"} {
		defs.CreatePackage(types.NewPackage("leaf", v, nil, nil))
	}
	for _, v := range []string{"1.0", "2.0"} {
		defs.CreatePackage(types.NewPackage("mid", v, leafReq, nil))
		defs.CreatePackage(types.NewPackage("top", v, midReq, nil))
	}

	installed.CreatePackage(types.NewPackage("leaf", "1.0", nil, nil))
	installed.CreatePackage(types.NewPackage("mid", "1.0", leafReq, nil))
	installed.CreatePackage(types.NewPackage("top", "1.0", midReq, nil))

	return defs, installed, solverdb
}

// TestUpgradeIsDeterministicAndNewest covers the reported symptom directly: an
// upgrade across a revdep chain, where several versions of the same package
// exist, must move everything to the newest and do so reproducibly.
func TestUpgradeIsDeterministicAndNewest(t *testing.T) {
	const want = "leaf-3.0 mid-2.0 top-2.0"

	seen := map[string]int{}
	for i := 0; i < runs; i++ {
		defs, installed, solverdb := upgradeWorld()
		s := newSolverFor(defs, installed, solverdb)

		_, asserts, err := s.Upgrade(false, true)
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("Upgrade produced %d distinct results over %d runs, want 1", len(seen), runs)
	}
	if n := seen[want]; n != runs {
		t.Errorf("upgraded to the newest set in %d of %d runs, want %d.\nobserved:", n, runs, runs)
		for r, c := range seen {
			t.Errorf("  %3d/%d  %s", c, runs, r)
		}
	}
}

// TestManyVersionsSelectsNewest is the narrow case that has regressed
// repeatedly: one package, many versions, nothing else to constrain the choice.
// With no conflicts and no competing requirements, the answer is unambiguous.
func TestManyVersionsSelectsNewest(t *testing.T) {
	versions := []string{"1.0", "1.5", "2.0", "2.10", "3.0", "10.0"}
	const want = "10.0"

	seen := map[string]int{}
	for i := 0; i < runs; i++ {
		defs := pkg.NewInMemoryDatabase(false)
		for _, v := range versions {
			defs.CreatePackage(types.NewPackage("only", v, nil, nil))
		}
		s := newSolverFor(defs, pkg.NewInMemoryDatabase(false), pkg.NewInMemoryDatabase(false))

		asserts, err := s.Install(types.Packages{
			types.NewPackage("only", ">=0", nil, nil),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		for _, a := range asserts {
			if a.Value && a.Package.GetName() == "only" {
				seen[a.Package.GetVersion()]++
			}
		}
	}

	if n := seen[want]; n != runs {
		t.Errorf("selected %q in %d of %d runs, want %d. Note 10.0 must beat 2.10 "+
			"and 3.0 - a string comparison would get this wrong.\nobserved: %v",
			want, n, runs, runs, seen)
	}
}

// TestRelaxedInstallSelectsNewest pins the path with no post-decode repair.
//
// Install re-solves its own result through computeUpgrade to pull versions up
// to Best. RelaxedInstall does not - it returns the decoded model directly - so
// it exposes the raw encoding. It is reachable from installer.go (--relaxed),
// from upgrade(), and from Uninstall(), so its version choice is not internal.
func TestRelaxedInstallSelectsNewest(t *testing.T) {
	const want = "3.0"

	seen := map[string]int{}
	for i := 0; i < runs; i++ {
		defs := pkg.NewInMemoryDatabase(false)
		for _, v := range []string{"1.0", "2.0", "3.0"} {
			defs.CreatePackage(types.NewPackage("dep", v, nil, nil))
		}
		defs.CreatePackage(types.NewPackage("root", "1.0",
			[]*types.Package{types.NewPackage("dep", ">=1.0", nil, nil)}, nil))

		s := newSolverFor(defs, pkg.NewInMemoryDatabase(false), pkg.NewInMemoryDatabase(false))
		asserts, err := s.RelaxedInstall(types.Packages{
			types.NewPackage("root", "1.0", nil, nil),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		for _, a := range asserts {
			if a.Value && a.Package.GetName() == "dep" {
				seen[a.Package.GetVersion()]++
			}
		}
	}

	if n := seen[want]; n != runs {
		t.Errorf("RelaxedInstall selected dep-%s in %d of %d runs, want %d.\nobserved: %v",
			want, n, runs, runs, seen)
	}
}

// TestBuildRevisionsSelectNewest exercises the rebuild counter BumpBuildVersion
// emits, end to end through the solver.
//
// This needs both halves of the fix and is the clearest demonstration of why
// they belong together. Ordering the candidates newest-first is only meaningful
// if "newest" is computed correctly, and until the comparators were unified it
// was not: SemVer treats everything after "+" as build metadata and excludes it
// from precedence, so 1.0, 1.0+1 and 1.0+2 all compared EQUAL. Sorting an
// all-equal list is a no-op, so the solver kept picking arbitrarily even with
// deterministic input.
//
// With one Debian comparator the three order properly, and the newest-first
// candidate order steers the solver to the latest rebuild.
func TestBuildRevisionsSelectNewest(t *testing.T) {
	const want = "1.0+2"

	seen := map[string]int{}
	for i := 0; i < runs; i++ {
		defs := pkg.NewInMemoryDatabase(false)
		for _, v := range []string{"1.0", "1.0+1", "1.0+2"} {
			defs.CreatePackage(types.NewPackage("rebuilt", v, nil, nil))
		}
		s := newSolverFor(defs, pkg.NewInMemoryDatabase(false), pkg.NewInMemoryDatabase(false))

		asserts, err := s.Install(types.Packages{
			types.NewPackage("rebuilt", ">=0", nil, nil),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		for _, a := range asserts {
			if a.Value && a.Package.GetName() == "rebuilt" {
				seen[a.Package.GetVersion()]++
			}
		}
	}

	if n := seen[want]; n != runs {
		t.Errorf("selected rebuilt-%s in %d of %d runs, want %d.\nobserved: %v",
			want, n, runs, runs, seen)
	}
}
