package solver_test

import (
	"fmt"
	"testing"
	"time"

	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/solver"
)

// The stress tests use versions 1.0..4.0, which are trivially ordered - a
// broken comparator would still pick 4.0. These use version shapes that
// actually distinguish a correct comparator from a plausible-looking one, and
// the expected answer is HARDCODED rather than computed from the versioner, so
// the test does not just ask the implementation to agree with itself.
type versionCase struct {
	name     string
	versions []string
	newest   string
	why      string
}

var versionCases = []versionCase{
	{
		name:     "double-digit",
		versions: []string{"1.0", "2.0", "9.0", "10.0"},
		newest:   "10.0",
		why:      "10 > 9 numerically; string comparison would pick 9.0",
	},
	{
		name:     "minor-double-digit",
		versions: []string{"1.2", "1.9", "1.10"},
		newest:   "1.10",
		why:      "1.10 > 1.9; string comparison would pick 1.9",
	},
	{
		name:     "build-revisions",
		versions: []string{"1.0", "1.0+1", "1.0+2", "1.0+10"},
		newest:   "1.0+10",
		why:      "BumpBuildVersion's +N counter; SemVer ignores it entirely",
	},
	{
		name:     "date-versions",
		versions: []string{"0.20191126", "0.20191205", "0.20191212"},
		newest:   "0.20191212",
		why:      "date-stamped versions as used in the complex fixture",
	},
	{
		name:     "tilde-prerelease",
		versions: []string{"1.0~rc1", "1.0~rc2", "1.0"},
		newest:   "1.0",
		why:      "a ~ prerelease sorts BEFORE its release",
	},
	{
		name:     "debian-revision",
		versions: []string{"1.0", "1.0-1", "1.0-2"},
		newest:   "1.0-2",
		why:      "a - revision sorts AFTER the bare version",
	},
	{
		name:     "gentoo-revision",
		versions: []string{"1.0", "1.0-r1", "1.0-r2"},
		newest:   "1.0-r2",
		why:      "Gentoo -rN revisions",
	},
	{
		name:     "depth",
		versions: []string{"1.0", "1.0.1", "1.0.10", "1.0.9"},
		newest:   "1.0.10",
		why:      "1.0.10 > 1.0.9 > 1.0.1 > 1.0",
	},
	{
		name:     "epoch",
		versions: []string{"1.0", "1.5", "2.0"},
		newest:   "2.0",
		why:      "plain control case alongside the epoch-bearing ones",
	},
	{
		name:     "mixed-shapes",
		versions: []string{"1.0", "1.0+1", "1.0-1", "1.0~rc1", "1.0.1"},
		newest:   "1.0.1",
		why:      "1.0.1 outranks revisions and build metadata of 1.0",
	},
}

// TestVersionOrderingThroughSolver checks that each shape resolves to the
// expected newest THROUGH the solver, not just through the comparator.
func TestVersionOrderingThroughSolver(t *testing.T) {
	const repeats = 10

	for _, tc := range versionCases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < repeats; i++ {
				defs := pkg.NewInMemoryDatabase(false)
				for _, v := range tc.versions {
					defs.CreatePackage(types.NewPackage("target", v, nil, nil))
				}

				s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
					pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

				asserts, err := s.Install(types.Packages{
					types.NewPackage("target", ">=0", nil, nil),
				})
				if err != nil {
					t.Fatalf("run %d: %s", i, err)
				}

				got := resolvedVersion(asserts, "target")
				if got != tc.newest {
					t.Fatalf("run %d: resolved %q, want %q\nversions: %v\nwhy: %s",
						i, got, tc.newest, tc.versions, tc.why)
				}
			}
		})
	}
}

// microVersions builds a dense version ladder for one family: many closely
// spaced versions, so ordering has to be right at every step rather than only
// at obvious boundaries.
//
// Shape, for depth=3 and micro=12:
//
//	1.0, 1.0.1 .. 1.0.12, 1.1, 1.1.1 .. 1.1.12, 1.2, ... then +N rebuilds of
//	the top one
//
// It deliberately crosses the 9/10 boundary in both the minor and micro
// positions, which is where a string comparison diverges from a numeric one.
func microVersions(majors, minors, micro, rebuilds int) []string {
	var out []string
	for maj := 1; maj <= majors; maj++ {
		for min := 0; min < minors; min++ {
			out = append(out, fmt.Sprintf("%d.%d", maj, min))
			for mic := 1; mic <= micro; mic++ {
				out = append(out, fmt.Sprintf("%d.%d.%d", maj, min, mic))
			}
		}
	}
	// Rebuilds of the highest version, which is what BumpBuildVersion emits.
	top := out[len(out)-1]
	for r := 1; r <= rebuilds; r++ {
		out = append(out, fmt.Sprintf("%s+%d", top, r))
	}
	return out
}

// newestMicroVersion is the expected answer, derived from the LADDER SHAPE
// rather than from the versioner, so the test states what newest means instead
// of asking the implementation to agree with itself.
func newestMicroVersion(majors, minors, micro, rebuilds int) string {
	top := fmt.Sprintf("%d.%d.%d", majors, minors-1, micro)
	if rebuilds > 0 {
		return fmt.Sprintf("%s+%d", top, rebuilds)
	}
	return top
}

// TestVersionOrderingDenseLadder checks a single family carrying a long, dense
// ladder of closely spaced versions - the case where an off-by-one in the
// comparator hides, because every neighbouring pair looks plausible.
func TestVersionOrderingDenseLadder(t *testing.T) {
	for _, cfg := range []struct{ majors, minors, micro, rebuilds int }{
		{2, 3, 12, 0},  // 1.0 .. 2.2.12, crosses 9/10 in micro
		{3, 12, 12, 0}, // crosses 9/10 in BOTH minor and micro
		{2, 3, 12, 5},  // ladder plus rebuild counters on top
		{1, 2, 40, 0},  // 40 micro versions in one minor
	} {
		name := fmt.Sprintf("%dx%dx%d+%d", cfg.majors, cfg.minors, cfg.micro, cfg.rebuilds)
		t.Run(name, func(t *testing.T) {
			versions := microVersions(cfg.majors, cfg.minors, cfg.micro, cfg.rebuilds)
			want := newestMicroVersion(cfg.majors, cfg.minors, cfg.micro, cfg.rebuilds)

			defs := pkg.NewInMemoryDatabase(false)
			for _, v := range versions {
				defs.CreatePackage(types.NewPackage("laddered", v, nil, nil))
			}

			s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
				pkg.NewInMemoryDatabase(false), defs, pkg.NewInMemoryDatabase(false))

			asserts, err := s.Install(types.Packages{
				types.NewPackage("laddered", ">=0", nil, nil),
			})
			if err != nil {
				t.Fatalf("%d versions: %s", len(versions), err)
			}

			if got := resolvedVersion(asserts, "laddered"); got != want {
				t.Fatalf("%d versions: resolved %q, want %q", len(versions), got, want)
			}
			t.Logf("%d versions in the ladder, newest correctly selected: %s", len(versions), want)
		})
	}
}

// TestVersionOrderingAtScale runs every shape above simultaneously in one large
// world with dependencies between families, so ordering is exercised where the
// solver has thousands of variables and real search to do - not one package in
// isolation.
func TestVersionOrderingAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short")
	}

	const copies = 200 // 200 x 10 shapes = 2000 families
	const repeats = 3

	seen := map[string]int{}

	for run := 0; run < repeats; run++ {
		defs := pkg.NewInMemoryDatabase(false)
		installed := pkg.NewInMemoryDatabase(false)

		type want struct{ name, version string }
		var expectations []want

		for c := 0; c < copies; c++ {
			for _, tc := range versionCases {
				name := fmt.Sprintf("%s-%d", tc.name, c)

				// Depend on a family from the previous copy, so the world is
				// connected and the solver must actually search.
				var requires []*types.Package
				if c > 0 {
					requires = append(requires, types.NewPackage(
						fmt.Sprintf("%s-%d", tc.name, c-1), ">=0", nil, nil))
				}

				for _, v := range tc.versions {
					defs.CreatePackage(types.NewPackage(name, v, requires, nil))
				}
				// Oldest listed version is what is installed, so everything is
				// upgradable.
				installed.CreatePackage(types.NewPackage(name, tc.versions[0], requires, nil))

				expectations = append(expectations, want{name, tc.newest})
			}
		}

		s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
			installed, defs, pkg.NewInMemoryDatabase(false))

		_, asserts, err := s.Upgrade(false, true)
		if err != nil {
			t.Fatalf("run %d: %s", run, err)
		}

		wrong := 0
		for _, w := range expectations {
			got := resolvedVersion(asserts, w.name)
			if got == "" {
				continue // not part of the upgrade set
			}
			if got != w.version {
				if wrong < 5 {
					t.Errorf("run %d: %s resolved to %q, want %q", run, w.name, got, w.version)
				}
				wrong++
			}
		}

		t.Logf("run %d: %d families, %d wrong", run, len(expectations), wrong)
		seen[installedVersions(asserts)]++
	}

	if len(seen) != 1 {
		t.Errorf("ordering is not stable: %d distinct results over %d runs", len(seen), repeats)
	}
}

// TestVersionOrderingLargeDenseWorld is the combined case: many families, each
// carrying a dense ladder of closely spaced versions, connected by
// dependencies.
//
// This is where the two risks meet. A long ladder means the at-most-one
// encoding is O(versions^2) per family, so the formula gets large fast; and
// closely spaced versions mean any comparator error picks a neighbour rather
// than something obviously wrong, which is exactly the kind of mistake that
// survives a small test.
//
// Every family's expected answer is derived from the ladder shape, not from the
// versioner.
//
// Sizes are kept modest on purpose, because resolution cost is dominated by
// VERSIONS PER FAMILY rather than by the number of families.
//
// Scaling families is linear (20 families x 26 versions measured at 1.6s,
// 40 at 3.4s). Scaling versions is not:
//
//	families   versions   packages   upgrade
//	      20         26        520      1.66s
//	      20         52       1040     10.00s
//	      20         78       1560     28.75s
//
// The at-most-one encoding is pairwise, so a family with V versions emits
// O(V^2) clauses, and BuildWorld regenerates them once per package rather than
// once per family. A package with a long release history - entirely ordinary in
// a real repository - therefore costs far more than its share.
//
// That is a genuine limitation, recorded here rather than hidden behind a
// smaller test. These cases verify ORDERING; the cost itself is a separate
// problem.
func TestVersionOrderingLargeDenseWorld(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short")
	}

	for _, cfg := range []struct {
		families, majors, minors, micro, rebuilds int
	}{
		{20, 1, 2, 12, 0}, // 20 families x 26 versions = 520 packages
		{40, 1, 2, 12, 0}, // 40 x 26                   = 1040 packages
		{20, 1, 2, 12, 3}, // 20 x 29, with rebuilds    = 580 packages
	} {
		versions := microVersions(cfg.majors, cfg.minors, cfg.micro, cfg.rebuilds)
		want := newestMicroVersion(cfg.majors, cfg.minors, cfg.micro, cfg.rebuilds)
		name := fmt.Sprintf("families=%d versions=%d", cfg.families, len(versions))

		t.Run(name, func(t *testing.T) {
			const repeats = 2
			seen := map[string]int{}

			for run := 0; run < repeats; run++ {
				start := time.Now()

				defs := pkg.NewInMemoryDatabase(false)
				installed := pkg.NewInMemoryDatabase(false)

				for i := 0; i < cfg.families; i++ {
					fam := fmt.Sprintf("fam%d", i)

					var requires []*types.Package
					if i > 0 {
						requires = append(requires, types.NewPackage(
							fmt.Sprintf("fam%d", i-1), ">=0", nil, nil))
					}

					for _, v := range versions {
						defs.CreatePackage(types.NewPackage(fam, v, requires, nil))
					}
					// Oldest in the ladder is installed, so all of it is upgradable.
					installed.CreatePackage(types.NewPackage(fam, versions[0], requires, nil))
				}
				build := time.Since(start)

				s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple},
					installed, defs, pkg.NewInMemoryDatabase(false))

				start = time.Now()
				_, asserts, err := s.Upgrade(false, true)
				upgrade := time.Since(start)
				if err != nil {
					t.Fatalf("run %d: %s", run, err)
				}

				checked, wrong := 0, 0
				for i := 0; i < cfg.families; i++ {
					got := resolvedVersion(asserts, fmt.Sprintf("fam%d", i))
					if got == "" {
						continue
					}
					checked++
					if got != want {
						if wrong < 3 {
							t.Errorf("run %d: fam%d resolved to %q, want %q", run, i, got, want)
						}
						wrong++
					}
				}

				t.Logf("run %d: %d packages | build=%s upgrade=%s | checked=%d wrong=%d",
					run, cfg.families*len(versions),
					build.Round(time.Millisecond), upgrade.Round(time.Millisecond),
					checked, wrong)

				seen[installedVersions(asserts)]++
			}

			if len(seen) != 1 {
				t.Errorf("ordering not stable: %d distinct results over %d runs",
					len(seen), repeats)
			}
		})
	}
}
