package solver_test

import (
	"fmt"
	"testing"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/solver"
)

// Search for a world where the current solver picks a version that is NOT the
// newest satisfiable one. Deterministic LCG so a hit is reproducible.
type rnd uint64

func (r *rnd) n(max int) int {
	*r = rnd(uint64(*r)*6364136223846793005 + 1442695040888963407)
	return int(*r>>33) % max
}

func searchSuboptimal(t *testing.T, optimize, relaxed bool) int {
	worlds := 400
	found := 0

	for w := 0; w < worlds; w++ {
		seed := rnd(w*7919 + 13)
		defs := pkg.NewInMemoryDatabase(false)

		nPkg := 4 + seed.n(4) // 4..7 package families
		nVer := 2 + seed.n(3) // 2..4 versions each
		names := []string{}
		for i := 0; i < nPkg; i++ {
			names = append(names, fmt.Sprintf("p%d", i))
		}

		type spec struct{ name, ver string }
		var all []spec
		for i, n := range names {
			for v := 1; v <= nVer; v++ {
				ver := fmt.Sprintf("%d.0", v)
				all = append(all, spec{n, ver})

				var req, conf []*types.Package
				// depend on a lower-indexed family
				if i > 0 && seed.n(100) < 70 {
					req = append(req, types.NewPackage(names[seed.n(i)], ">=1.0", nil, nil))
				}
				// newer versions more likely to conflict with something
				if i > 0 && v > 1 && seed.n(100) < 45 {
					conf = append(conf, types.NewPackage(
						names[seed.n(i)], fmt.Sprintf("%d.0", 1+seed.n(nVer)), nil, nil))
				}
				defs.CreatePackage(types.NewPackage(n, ver, req, conf))
			}
		}

		s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple, Optimize: optimize},
			pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

		target := names[len(names)-1]
		var asserts types.PackagesAssertions
		var err error
		if relaxed {
			asserts, err = s.RelaxedInstall(types.Packages{types.NewPackage(target, ">=0", nil, nil)})
		} else {
			asserts, err = s.Install(types.Packages{types.NewPackage(target, ">=0", nil, nil)})
		}
		if err != nil {
			continue // UNSAT worlds are not interesting here
		}

		// For each installed package, is a strictly newer version of it also
		// installable given the rest of this solution? Approximate by checking
		// whether the newest version exists and was not chosen.
		for _, a := range asserts {
			if !a.Value {
				continue
			}
			versions, err := defs.FindPackageVersions(a.Package)
			if err != nil || len(versions) == 0 {
				continue
			}
			newest := versions.Best(nil)
			if newest.GetVersion() == a.Package.GetVersion() {
				continue
			}

			// The newest merely EXISTING is not enough - it may be genuinely
			// blocked. Prove it was installable by re-solving with it forced
			// alongside the original request. If that succeeds, the first
			// solution was needlessly old.
			s2 := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
				pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))
			if _, err := s2.Install(types.Packages{
				types.NewPackage(target, ">=0", nil, nil),
				types.NewPackage(newest.GetName(), newest.GetVersion(), nil, nil),
			}); err != nil {
				continue // genuinely blocked, correctly not chosen
			}

			t.Logf("world %d: %s chose %s, but %s was installable",
				w, a.Package.GetName(), a.Package.GetVersion(), newest.GetVersion())
			found++
			break
		}
	}

	t.Logf("=== optimize=%v relaxed=%v: non-newest picks: %d / %d ===", optimize, relaxed, found, worlds)
	return found
}

// TestOptimizeImprovesVersionSelection measures the experimental improvement
// pass over 400 generated worlds, and asserts it does what it claims.
//
// A world counts against us only when a re-solve PROVES a newer version was
// installable alongside the rest of the solution - not merely that one exists.
// Legitimately blocked versions are not failures.
//
// Measured at the time of writing:
//
//	Install         off=64/400   on=39/400
//	RelaxedInstall  off=400/400  on=40/400
//
// Install already runs a repair pass (computeUpgrade + re-solve) that pulls
// versions up, which is why its baseline is low. RelaxedInstall has no such
// pass and so shows the raw encoding: every world settled on a needlessly old
// version. The improvement pass brings it to parity with Install.
//
// The residual ~10% is inherent to the greedy strategy: each accepted
// improvement is kept as a constraint, so pulling one package up early can
// block a larger gain later. Closing that gap needs a real optimisation
// objective, which is what MaxSAT would provide and why it was measured and
// rejected (>20000x slower on the vendored gophersat).
func TestOptimizeImprovesVersionSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 400-world measurement under -short")
	}

	relaxedOff := searchSuboptimal(t, false, true)
	relaxedOn := searchSuboptimal(t, true, true)
	installOff := searchSuboptimal(t, false, false)
	installOn := searchSuboptimal(t, true, false)

	t.Logf("RESULT Install         off=%d on=%d", installOff, installOn)
	t.Logf("RESULT RelaxedInstall  off=%d on=%d", relaxedOff, relaxedOn)

	// The pass must substantially improve the path that has no repair pass.
	// Generous threshold - this guards against the feature silently breaking,
	// not against small drifts in the generator.
	if relaxedOn*2 >= relaxedOff {
		t.Errorf("Optimize barely helped RelaxedInstall: %d -> %d of 400 worlds; "+
			"expected at least a halving", relaxedOff, relaxedOn)
	}

	// It must never make things worse.
	if installOn > installOff {
		t.Errorf("Optimize made Install worse: %d -> %d of 400 worlds",
			installOff, installOn)
	}
}
