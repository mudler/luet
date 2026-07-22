package solver_test

import (
	"testing"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/solver"
)

// A dependency on a package the definition database has never heard of must NOT
// be fatal, while a dependency on a package it DOES know but cannot satisfy
// must be.
//
// FindPackages distinguishes the two and callers have to respect it:
//
//	err != nil              the name is unknown here
//	err == nil, len == 0    the name is known, no version satisfies the range
//
// Real trees depend on the first case being tolerated. A repository is one of
// several a system has configured, and a package may name a dependency provided
// by a repository that is not synced at the point the formula is built. Treating
// that as unsatisfiable broke installs on mocaccinoOS/desktop:
//
//	Failed solving solution for package:
//	  no packages satisfy entity/audio->=0, required by layers/X-26.07+5
//
// ">=0" matches any version, so that error can only mean the name was absent
// entirely - never that a range went unmet.
func TestUnknownDependencyIsTolerated(t *testing.T) {
	defs := pkg.NewInMemoryDatabase(false)

	// Requires a package that does not exist in this database at all.
	defs.CreatePackage(types.NewPackage("app", "1.0",
		[]*types.Package{types.NewPackage("elsewhere", ">=0", nil, nil)}, nil))

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	if _, err := s.Install(types.Packages{types.NewPackage("app", "1.0", nil, nil)}); err != nil {
		t.Fatalf("a dependency on a package unknown to this database must not be "+
			"fatal - it may come from another repository: %s", err)
	}
}

// TestUnsatisfiableRangeStillErrors keeps the phantom fix honest: when the name
// IS known and no version satisfies the range, the request must fail rather
// than encode the selector as a package that does not exist.
func TestUnsatisfiableRangeStillErrors(t *testing.T) {
	defs := pkg.NewInMemoryDatabase(false)
	defs.CreatePackage(types.NewPackage("known", "1.0", nil, nil))
	defs.CreatePackage(types.NewPackage("known", "2.0", nil, nil))
	defs.CreatePackage(types.NewPackage("app", "1.0",
		[]*types.Package{types.NewPackage("known", ">=99.0", nil, nil)}, nil))

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	asserts, err := s.Install(types.Packages{types.NewPackage("app", "1.0", nil, nil)})
	for _, a := range asserts {
		if a.Value && a.Package.IsSelector() {
			t.Errorf("solution contains a selector rather than a real package: %s-%s",
				a.Package.GetName(), a.Package.GetVersion())
		}
	}
	if err == nil {
		t.Fatal("a known package with no version satisfying the range must be an error")
	}
}

// The same distinction has to hold for a TOP-LEVEL request, not just for a
// transitive dependency.
//
// getList, resolveWanted and wantedFormula each resolve a requested selector,
// and each treated "no candidates" as fatal regardless of why. A name unknown
// to this database is not evidence that the request is impossible - it is the
// same situation that broke installs when a dependency named a package from a
// repository not yet in scope.
func TestUnknownTopLevelSelectorIsTolerated(t *testing.T) {
	defs := pkg.NewInMemoryDatabase(false)
	defs.CreatePackage(types.NewPackage("present", "1.0", nil, nil))

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	// A selector naming a package this database has never seen.
	if _, err := s.Install(types.Packages{
		types.NewPackage("absent", ">=0", nil, nil),
	}); err != nil {
		t.Fatalf("a request for a package unknown to this database must not be "+
			"fatal - it may come from another repository: %s", err)
	}
}

// TestKnownTopLevelUnsatisfiableStillErrors is the other half: the name is
// known, so a range nothing satisfies is a real failure and must not resolve to
// a selector atom.
func TestKnownTopLevelUnsatisfiableStillErrors(t *testing.T) {
	defs := pkg.NewInMemoryDatabase(false)
	defs.CreatePackage(types.NewPackage("known", "1.0", nil, nil))
	defs.CreatePackage(types.NewPackage("known", "2.0", nil, nil))

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	asserts, err := s.Install(types.Packages{
		types.NewPackage("known", ">=99.0", nil, nil),
	})
	for _, a := range asserts {
		if a.Value && a.Package.IsSelector() {
			t.Errorf("solution contains a selector rather than a real package: %s-%s",
				a.Package.GetName(), a.Package.GetVersion())
		}
	}
	if err == nil {
		t.Fatal("a known package with no version satisfying the range must be an error")
	}
}

// TestMissingDependencyFixtureReproducesFieldFailure is the regression guard for
// a real breakage, reproduced through a real tree rather than a hand-built
// database.
//
// Installs on mocaccinoOS/desktop failed with:
//
//	Failed solving solution for package:
//	  no packages satisfy entity/audio->=0, required by layers/X-26.07+5
//
// tests/fixtures/missingdep mirrors that shape exactly - a layers/X carrying a
// build revision, requiring two entity/* packages by ">=0", of which only one is
// present in the tree.
//
// The distinction that matters: ">=0" matches ANY version, so it cannot fail
// because a range went unmet. It can only fail when the name is absent from this
// database - which is not evidence the request is impossible, since a system has
// several repositories and a formula may be built before all of them are in
// scope.
//
// The fixture path matters too. The database is populated by the recipe loader
// from YAML, so selector expansion goes through the same version cache the real
// pipeline uses. A hand-built database can be made to look right while the tree
// that produces it does not.
func TestMissingDependencyFixtureReproducesFieldFailure(t *testing.T) {
	defs := loadFixtureTree(t, "../../tests/fixtures/missingdep")

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	asserts, err := s.Install(types.Packages{
		withCategory("layers", "X", "26.07+5"),
	})
	if err != nil {
		t.Fatalf("installing a package whose dependency is absent from this tree "+
			"must not fail - entity/audio may come from another repository: %s", err)
	}

	// The dependency that IS present must still be resolved, so the relaxation
	// does not quietly disable dependency handling for the whole package.
	if got := resolvedVersion(asserts, "video"); got != "1.0" {
		t.Errorf("entity/video resolved to %q, want 1.0 - relaxing the missing "+
			"dependency must not drop the satisfiable ones", got)
	}
}
