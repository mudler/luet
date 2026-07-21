package solver_test

import (
	"fmt"
	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	"testing"
	"time"
)

// Stress the solver at repository scale. Everything else in this suite runs on
// worlds of a handful of packages; a real tree is thousands, and the properties
// that matter (determinism, newest-selection) are emergent from CNF variable
// ordering, so they need checking where clause learning actually has room to
// work.
//
// Run with: go test -run TestStress -timeout 60m ./pkg/solver/
func TestStressLargeWorld(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}

	for _, n := range []int{500, 1000, 2000} {
		t.Run(fmt.Sprintf("packages=%d", n), func(t *testing.T) {
			o := defaultWorld(n)

			start := time.Now()
			defs, installed, solverdb := buildWorld(o)
			build := time.Since(start)

			s := newBenchSolver(defs, installed, solverdb)

			start = time.Now()
			_, asserts, err := s.Upgrade(false, true)
			upgrade := time.Since(start)
			if err != nil {
				t.Fatalf("upgrade failed: %s", err)
			}

			installedCount, stale := 0, 0
			for _, a := range asserts {
				if !a.Value {
					continue
				}
				installedCount++
				versions, err := defs.FindPackageVersions(a.Package)
				if err != nil || len(versions) == 0 {
					continue
				}
				if versions.Best(nil).GetVersion() != a.Package.GetVersion() {
					stale++
				}
			}

			t.Logf("build=%s upgrade=%s assertions=%d stale=%d (%.1f%%)",
				build.Round(time.Millisecond), upgrade.Round(time.Millisecond),
				installedCount, stale, 100*float64(stale)/float64(max(installedCount, 1)))
		})
	}
}

// TestStressDeterminismAtScale re-solves a large world repeatedly. Map-iteration
// nondeterminism is far likelier to show up with thousands of variables than
// with ten.
func TestStressDeterminismAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}

	const n = 1000
	const repeats = 5

	seen := map[string]int{}
	for i := 0; i < repeats; i++ {
		defs, installed, solverdb := buildWorld(defaultWorld(n))
		s := newBenchSolver(defs, installed, solverdb)

		_, asserts, err := s.Upgrade(false, true)
		if err != nil {
			t.Fatalf("run %d: %s", i, err)
		}
		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("a %d-package world gave %d distinct results over %d runs, want 1",
			n, len(seen), repeats)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildLargeWorld generates a repository-scale world with real structure:
// several versions per family, dense layered dependencies expressed as
// selectors, and conflicts that actually bite.
//
// The conflicts are the point. Every other generated world in this suite is
// conflict-free, so the solver never backtracks and clause learning never
// starts driving variable activity - which is exactly the regime where the
// newest-first ordering is only a heuristic. Here the NEWEST version of some
// families conflicts with the NEWEST version of an earlier family, so the
// solver must give ground on one of them. The world stays satisfiable (taking
// the older version of either side resolves it) but reaching a solution
// requires real search.
func buildLargeWorld(families, versions, conflictPct int) (types.PackageDatabase, types.PackageDatabase, types.PackageDatabase) {
	defs := pkg.NewInMemoryDatabase(false)
	installed := pkg.NewInMemoryDatabase(false)
	solverdb := pkg.NewInMemoryDatabase(false)

	r := lcg(20260721)
	newest := fmt.Sprintf("%d.0", versions)

	for i := 0; i < families; i++ {
		name := fmt.Sprintf("pkg%d", i)

		// Dependencies point strictly backwards, so the graph stays acyclic.
		var requires []*types.Package
		if i > 0 {
			for d := 0; d < 1+r.next(5); d++ { // 1..5 dependencies
				requires = append(requires, types.NewPackage(
					fmt.Sprintf("pkg%d", r.next(i)), ">=1.0", nil, nil))
			}
		}

		for v := 1; v <= versions; v++ {
			version := fmt.Sprintf("%d.0", v)

			// Only the newest version carries a conflict, and it targets the
			// version that is currently INSTALLED (1.0) rather than another
			// newest. That models the real case - a new release that refuses to
			// coexist with an old one - and keeps the world satisfiable, since
			// upgrading the other family away from 1.0 resolves it.
			//
			// Pointing conflicts at other NEWEST versions instead makes the
			// world unsatisfiable by construction: an upgrade drives everything
			// to newest at once, so two mutually exclusive newests can never
			// both be installed. Worth knowing, because proving that UNSAT is
			// expensive - measured at 1.3s/6.2s/20.6s for 100/200/400 families,
			// and beyond 90s at 800.
			var conflicts []*types.Package
			if i > 0 && version == newest && r.next(100) < conflictPct {
				conflicts = append(conflicts, types.NewPackage(
					fmt.Sprintf("pkg%d", r.next(i)), "1.0", nil, nil))
			}

			defs.CreatePackage(types.NewPackage(name, version, requires, conflicts))
			if v == 1 {
				installed.CreatePackage(types.NewPackage(name, version, requires, nil))
			}
		}
	}

	return defs, installed, solverdb
}

// TestStressLargeConnectedWorld is the repository-scale run: 2000 families,
// 4 versions each (8000 packages), 1-5 dependencies per family, and conflicts
// on 10% of the newest versions.
func TestStressLargeConnectedWorld(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}

	const (
		families    = 2000
		versions    = 4
		conflictPct = 10
		repeats     = 3
	)

	seen := map[string]int{}
	var lastStale, lastTotal int

	for i := 0; i < repeats; i++ {
		start := time.Now()
		defs, installed, solverdb := buildLargeWorld(families, versions, conflictPct)
		build := time.Since(start)

		s := newBenchSolver(defs, installed, solverdb)

		start = time.Now()
		_, asserts, err := s.Upgrade(false, true)
		upgrade := time.Since(start)
		if err != nil {
			t.Fatalf("run %d: upgrade failed after %s: %s", i, upgrade, err)
		}

		total, stale := 0, 0
		for _, a := range asserts {
			if !a.Value {
				continue
			}
			total++
			vs, err := defs.FindPackageVersions(a.Package)
			if err != nil || len(vs) == 0 {
				continue
			}
			if vs.Best(nil).GetVersion() != a.Package.GetVersion() {
				stale++
			}
		}
		lastStale, lastTotal = stale, total

		t.Logf("run %d: %d families x %d versions (%d packages) | build=%s upgrade=%s | "+
			"installed=%d not-newest=%d (%.1f%%)",
			i, families, versions, families*versions,
			build.Round(time.Millisecond), upgrade.Round(time.Millisecond),
			total, stale, 100*float64(stale)/float64(max(total, 1)))

		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("DETERMINISM: %d distinct results over %d runs of an identical "+
			"%d-package world, want 1", len(seen), repeats, families*versions)
	}

	// Not an assertion on the exact figure - with conflicts present, some
	// packages are legitimately held back. This guards against a collapse.
	if ratio := float64(lastStale) / float64(max(lastTotal, 1)); ratio > 0.5 {
		t.Errorf("more than half the packages are not at their newest version "+
			"(%d/%d); the ordering heuristic has likely regressed", lastStale, lastTotal)
	}
}
