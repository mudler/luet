package solver_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/solver"
)

// unsatWorld: newest versions conflict with each other, so an upgrade that
// drives everything to newest is unsatisfiable.
func unsatWorld(families, versions, conflictPct int) (types.PackageDatabase, types.PackageDatabase, types.PackageDatabase) {
	defs := pkg.NewInMemoryDatabase(false)
	installed := pkg.NewInMemoryDatabase(false)
	solverdb := pkg.NewInMemoryDatabase(false)
	r := lcg(20260721)
	newest := fmt.Sprintf("%d.0", versions)

	for i := 0; i < families; i++ {
		name := fmt.Sprintf("pkg%d", i)
		var requires []*types.Package
		if i > 0 {
			for d := 0; d < 1+r.next(5); d++ {
				requires = append(requires, types.NewPackage(fmt.Sprintf("pkg%d", r.next(i)), ">=1.0", nil, nil))
			}
		}
		for v := 1; v <= versions; v++ {
			version := fmt.Sprintf("%d.0", v)
			var conflicts []*types.Package
			if i > 0 && version == newest && r.next(100) < conflictPct {
				conflicts = append(conflicts, types.NewPackage(fmt.Sprintf("pkg%d", r.next(i)), newest, nil, nil))
			}
			defs.CreatePackage(types.NewPackage(name, version, requires, conflicts))
			if v == 1 {
				installed.CreatePackage(types.NewPackage(name, version, requires, nil))
			}
		}
	}
	return defs, installed, solverdb
}

// TestUnsatIsReportedPromptly guards the failure path.
//
// An unsatisfiable upgrade must fail quickly. The solver itself proves UNSAT in
// linear time; the cost used to be entirely in Explainer's MUS extraction,
// which is roughly quadratic in clause count and runs only on failure:
//
//	families   solve    with MUS (before)   after
//	     100    244ms              1.30s    256ms
//	     200    419ms              5.61s    474ms
//	     400    949ms             19.94s    940ms
//	     800     1.9s      >90s abandoned    1.9s
//
// A real repository is well past the last row, so a genuine conflict looked
// like a hang rather than an error. Explanations are still produced below
// maxExplainClauses, where they cost little - see TestUnsatIsExplainedWhenSmall.
func TestUnsatIsReportedPromptly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short")
	}

	const budget = 30 * time.Second

	for _, n := range []int{200, 400, 800} {
		defs, installed, solverdb := unsatWorld(n, 4, 10)
		s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, installed, defs, solverdb)

		start := time.Now()
		_, _, err := s.Upgrade(false, true)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatalf("n=%d: expected an unsatisfiable world", n)
		}
		t.Logf("n=%d families: failed in %s", n, elapsed.Round(time.Millisecond))

		if elapsed > budget {
			t.Errorf("n=%d: took %s to report UNSAT, over the %s budget - the "+
				"explanation bound has likely regressed", n, elapsed.Round(time.Millisecond), budget)
		}
	}
}

// TestUnsatIsExplainedWhenSmall pins the other half: below the clause bound the
// error still names the constraints that could not be met, which is the whole
// point of the Explainer.
func TestUnsatIsExplainedWhenSmall(t *testing.T) {
	defs := pkg.NewInMemoryDatabase(false)
	defs.CreatePackage(types.NewPackage("a", "1.0",
		[]*types.Package{types.NewPackage("b", "1.0", nil, nil)}, nil))
	defs.CreatePackage(types.NewPackage("b", "1.0", nil,
		[]*types.Package{types.NewPackage("a", "1.0", nil, nil)}))

	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
		pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

	_, err := s.Install(types.Packages{types.NewPackage("a", "1.0", nil, nil)})
	if err == nil {
		t.Fatal("expected the mutual a/b conflict to be unsatisfiable")
	}
	if !strings.Contains(err.Error(), "a--1.0") {
		t.Errorf("small failures should still be explained, got: %s", err)
	}
}
