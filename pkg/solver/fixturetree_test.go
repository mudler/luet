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
	"testing"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	"github.com/mudler/luet/pkg/tree"

	. "github.com/mudler/luet/pkg/solver"
)

// These exercise resolution against a real fixture tree - YAML on disk, parsed
// by the recipe loader into a database, then solved - rather than a database
// assembled by hand.
//
// That matters because the paths under test only exist in the real pipeline.
// Selector expansion consults the version cache the loader populates, and the
// relax-on-failure branches in buildFormula and getList are reached through
// FindPackages, not through direct construction. A hand-built world can be made
// to look right while the tree that produces it does not.
//
// tests/fixtures/complex is used because it has the shape that matters: several
// packages carrying more than one version (sabayon-build-portage at 0.20191126
// and 0.20191212, build-sabayon-overlay at 0.20191205 and 0.20191212,
// build-sabayon-overlays at 0.1 and 0.20191212) and dependencies expressed as
// ">=" selectors against them.

func loadFixtureTree(t *testing.T, path string) types.PackageDatabase {
	t.Helper()

	db := pkg.NewInMemoryDatabase(false)
	recipe := tree.NewCompilerRecipe(db)
	if err := recipe.Load(path); err != nil {
		t.Fatalf("loading %s: %s", path, err)
	}
	return recipe.GetDatabase()
}

// withCategory builds a request package. Category is part of package identity,
// so a request without one matches nothing in a real tree.
func withCategory(category, name, version string) *types.Package {
	p := types.NewPackage(name, version, nil, nil)
	p.Category = category
	return p
}

func solverFor(defs types.PackageDatabase) types.PackageSolver {
	return NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))
}

// resolvedVersion returns the version selected for a package name, or "" if the
// name is absent from the solution.
func resolvedVersion(asserts types.PackagesAssertions, name string) string {
	for _, a := range asserts {
		if a.Value && a.Package.GetName() == name {
			return a.Package.GetVersion()
		}
	}
	return ""
}

// TestFixtureTreeSelectsNewestVersion solves a real tree and checks that
// multi-version dependencies resolve to their newest.
//
// In tests/fixtures/complex, sabayon-sources pins build-sabayon-overlays to
// 0.1, which in turn pins sabayon-build-portage to 0.20191126 exactly while
// leaving build-sabayon-overlay open as ">=0.1". Both of those names ship two
// versions, so a correct solve must pick the newest where it is free to and
// honour the pin where it is not - in the same run.
func TestFixtureTreeSelectsNewestVersion(t *testing.T) {
	const iterations = 30

	// Both of these ship two versions in the fixture, and the tree constrains
	// them differently - which is the point. One is a free choice among
	// candidates, the other is pinned, and a correct solve has to honour both
	// in the same run.
	expected := map[string]string{
		// Reached via "build-sabayon-overlay >=0.1", so the solver chooses -
		// and must choose the newer of 0.20191205 / 0.20191212.
		"build-sabayon-overlay": "0.20191212",

		// NOT a free choice: build-sabayon-overlays-0.1 requires
		// "sabayon-build-portage 0.20191126" exactly, so the older version is
		// the correct answer even though 0.20191212 exists. Preferring the
		// newest must not override an exact pin.
		"sabayon-build-portage": "0.20191126",
	}

	for i := 0; i < iterations; i++ {
		defs := loadFixtureTree(t, "../../tests/fixtures/complex")
		s := solverFor(defs)

		asserts, err := s.Install(types.Packages{
			// The category is required. Without it the request matches nothing
			// and, before the phantom fix, silently resolved to a selector atom
			// rather than failing - which made an earlier version of this test
			// pass vacuously.
			withCategory("sys-kernel", "sabayon-sources", ">=0"),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}

		for name, want := range expected {
			got := resolvedVersion(asserts, name)
			if got == "" {
				t.Fatalf("run %d: %s is absent from the solution; the assertion "+
					"would pass vacuously", i, name)
			}
			if got != want {
				t.Fatalf("run %d: %s resolved to %s, want %s (the newest in the tree)",
					i, name, got, want)
			}
		}
	}
}

// TestFixtureTreeResolutionIsDeterministic solves the same real tree repeatedly
// and requires an identical answer each time.
func TestFixtureTreeResolutionIsDeterministic(t *testing.T) {
	const iterations = 30

	seen := map[string]int{}
	for i := 0; i < iterations; i++ {
		defs := loadFixtureTree(t, "../../tests/fixtures/complex")
		s := solverFor(defs)

		asserts, err := s.Install(types.Packages{
			withCategory("sys-kernel", "sabayon-sources", ">=0"),
		})
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("solving the same tree gave %d distinct results over %d runs, want 1:",
			len(seen), iterations)
		for r, n := range seen {
			t.Errorf("  %2d/%d  %s", n, iterations, r)
		}
	}
}

// TestFixtureTreeUnsatisfiableSelectorErrors is the soundness case.
//
// Asking for a version of a real package that the tree does not contain must
// FAIL. Today it succeeds, and the returned solution contains an assertion for
// a package that does not exist:
//
//	Install(sabayon-sources >= 99.0) -> err <nil>
//	  sabayon-sources->=99.0   Value=true, IsSelector()=true
//
// The mechanism: FindPackages returns (nil, nil) - empty, no error - when a
// package name is known but no version satisfies the range. The caller cannot
// distinguish "no such package" from "package exists, range unsatisfiable", and
// treats both as licence to encode the SELECTOR ITSELF as a SAT variable. That
// variable has no at-least-one clause tying it to any concrete version, so the
// solver can satisfy it freely, while the at-most-one clauses it appears in
// actively forbid installing any real version of the package.
//
// The result is worse than a missing constraint: the resolver reports success
// and hands the installer something uninstallable.
func TestFixtureTreeUnsatisfiableSelectorErrors(t *testing.T) {
	defs := loadFixtureTree(t, "../../tests/fixtures/complex")
	s := solverFor(defs)

	asserts, err := s.Install(types.Packages{
		withCategory("sys-kernel", "sabayon-sources", ">=99.0"),
	})

	for _, a := range asserts {
		if a.Value && a.Package.IsSelector() {
			t.Errorf("solution contains a selector, not a concrete package: %s-%s",
				a.Package.GetName(), a.Package.GetVersion())
		}
	}

	if err == nil {
		t.Fatal("Install succeeded for a version that does not exist in the tree; " +
			"an unsatisfiable selector must be an error, not a phantom package")
	}
}

// TestFixtureTreeUnsatisfiableDependencyErrors covers the same defect reached
// through a dependency rather than a top-level request.
//
// buildFormula takes the same relax-on-failure branch when a package's
// `requires` names a version that is not in the tree, so the phantom is
// encoded mid-graph where it is even less visible.
func TestFixtureTreeUnsatisfiableDependencyErrors(t *testing.T) {
	defs := loadFixtureTree(t, "../../tests/fixtures/complex")

	// A package whose dependency cannot be satisfied by anything in the tree.
	if _, err := defs.CreatePackage(types.NewPackage("broken", "1.0",
		[]*types.Package{withCategory("layer", "build", ">=99.0")}, nil)); err != nil {
		t.Fatal(err)
	}

	s := solverFor(defs)
	asserts, err := s.Install(types.Packages{
		types.NewPackage("broken", "1.0", nil, nil),
	})

	for _, a := range asserts {
		if a.Value && a.Package.IsSelector() {
			t.Errorf("solution contains a selector, not a concrete package: %s-%s",
				a.Package.GetName(), a.Package.GetVersion())
		}
	}

	if err == nil {
		t.Fatal("Install succeeded despite an unsatisfiable dependency")
	}
}

// TestFixtureTreeSelectorBacktracks is the false-UNSAT case.
//
// The tree in tests/fixtures/selectorbacktrack is:
//
//	base    1.0
//	base    2.0   conflicts with pinned 1.0
//	pinned  1.0
//	app     1.0   requires pinned 1.0
//
// Installing app and "base >= 1.0" together has exactly one solution:
// app pulls in pinned-1.0, base-2.0 conflicts with it, so base must be 1.0.
//
// getList collapses a top-level selector to a single candidate with Best()
// BEFORE the formula is built. That hands the solver base-2.0 as a hard unit
// clause, the conflict makes it unsatisfiable, and luet reports the whole
// request unsolvable - even though base-1.0 satisfies it.
//
// The alternatives have to survive into the formula for the solver to back off.
// Note this affects only TOP-LEVEL requests: buildFormula already encodes
// at-least-one over all candidates for a `requires` entry, so the same shape
// reached through a dependency resolves correctly today.
func TestFixtureTreeSelectorBacktracks(t *testing.T) {
	const iterations = 20

	for i := 0; i < iterations; i++ {
		defs := loadFixtureTree(t, "../../tests/fixtures/selectorbacktrack")
		s := solverFor(defs)

		asserts, err := s.Install(types.Packages{
			withCategory("test", "app", "1.0"),
			withCategory("test", "base", ">=1.0"),
		})
		if err != nil {
			t.Fatalf("run %d: reported unsolvable, but base-1.0 satisfies the "+
				"request: %s", i, err)
		}

		if got := resolvedVersion(asserts, "base"); got != "1.0" {
			t.Fatalf("run %d: base resolved to %q, want 1.0 - 2.0 conflicts with "+
				"pinned, which app requires", i, got)
		}
		if got := resolvedVersion(asserts, "pinned"); got != "1.0" {
			t.Fatalf("run %d: pinned resolved to %q, want 1.0", i, got)
		}
	}
}
