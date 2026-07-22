package solver_test

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/crillab/gophersat/bf"
	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/solver"
)

// formulaSize reports the variable and clause counts of the world formula, read
// from its DIMACS header.
func formulaSize(t *testing.T, s types.PackageSolver) (vars, clauses int) {
	t.Helper()

	f, err := s.(*Solver).BuildWorld(false)
	if err != nil {
		t.Fatalf("building world: %s", err)
	}

	buf := bytes.NewBufferString("")
	if err := bf.Dimacs(f, buf); err != nil {
		t.Fatalf("dimacs: %s", err)
	}

	for _, line := range strings.Split(buf.String(), "\n") {
		if !strings.HasPrefix(line, "p cnf ") {
			continue
		}
		fields := strings.Fields(line)
		vars, _ = strconv.Atoi(fields[2])
		clauses, _ = strconv.Atoi(fields[3])
		return vars, clauses
	}
	t.Fatal("no DIMACS header found")
	return 0, 0
}

// oneFamilyWorld builds a single package family with n versions and no
// dependencies, so the formula contains nothing but the mutual-exclusion
// clauses between versions.
func oneFamilyWorld(n int) types.PackageDatabase {
	defs := pkg.NewInMemoryDatabase(false)
	for v := 1; v <= n; v++ {
		defs.CreatePackage(types.NewPackage("solo", fmt.Sprintf("%d.0", v), nil, nil))
	}
	return defs
}

// TestVersionExclusionClauseCount pins the size of the mutual-exclusion
// encoding.
//
// Versions of a package are mutually exclusive, which needs one clause per
// unordered PAIR: n*(n-1)/2. buildFormula runs once per package in the world
// and emits a clause for every OTHER version, so each pair is emitted twice -
// once from each end. The formula is still correct, just twice the size it
// needs to be, and this is the term that dominates: cost measured at 1.66s /
// 10.0s / 28.7s for 26 / 52 / 78 versions per family.
func TestVersionExclusionClauseCount(t *testing.T) {
	for _, n := range []int{10, 20, 40} {
		t.Run(fmt.Sprintf("versions=%d", n), func(t *testing.T) {
			s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
				pkg.NewInMemoryDatabase(false), oneFamilyWorld(n), pkg.NewInMemoryDatabase(false))

			vars, clauses := formulaSize(t, s)

			pairs := n * (n - 1) / 2
			t.Logf("n=%d vars=%d clauses=%d (unordered pairs = %d, ratio %.2f)",
				n, vars, clauses, pairs, float64(clauses)/float64(pairs))

			// One clause per unordered pair, with a little slack for the
			// bookkeeping bf adds around the conjunction.
			if clauses > pairs+n {
				t.Errorf("n=%d: %d clauses for %d unordered pairs - each pair is "+
					"being emitted more than once", n, clauses, pairs)
			}
		})
	}
}

// TestEncodeIsCheapWhenAlreadyStored pins the cost of encoding a package that
// the solver database already holds.
//
// Encode assigns the SAT variable name for a package, and formula construction
// calls it for every reference - once per literal, in loops that are quadratic
// in the number of versions. It routes through CreatePackage, which marshalled
// the package to JSON and base64-encoded it before checking whether the entry
// already existed, so the marshal was paid on every reference rather than once.
//
// Profiling a dense-version upgrade showed the result: GC dominated the run,
// with json.structEncoder.encode and base64.Encode as the largest identifiable
// contributors.
func TestEncodeIsCheapWhenAlreadyStored(t *testing.T) {
	db := pkg.NewInMemoryDatabase(false)

	// A package with dependencies, as real ones have. Marshalling cost scales
	// with the struct's contents, so a bare package understates it.
	var requires []*types.Package
	for i := 0; i < 6; i++ {
		requires = append(requires, types.NewPackage(fmt.Sprintf("dep%d", i), ">=1.0", nil, nil))
	}
	p := types.NewPackage("cached", "1.0", requires, nil)

	if _, err := p.Encode(db); err != nil {
		t.Fatal(err)
	}

	allocs := testing.AllocsPerRun(200, func() {
		if _, err := p.Encode(db); err != nil {
			t.Fatal(err)
		}
	})

	t.Logf("re-encoding an already stored package: %.0f allocations", allocs)

	// A lookup plus the fingerprint string. Marshalling a package with
	// dependencies allocates an order of magnitude more.
	if allocs > 3 {
		t.Errorf("re-encoding allocates %.0f objects - it is re-marshalling "+
			"rather than reusing the stored entry", allocs)
	}
}
