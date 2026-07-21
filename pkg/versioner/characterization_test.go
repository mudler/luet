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

package version_test

import (
	"testing"
	"time"

	. "github.com/mudler/luet/pkg/versioner"
)

// CHARACTERIZATION TESTS
//
// These record what the version code does TODAY, bugs included. They are not
// a statement of what it should do. Their purpose is to make the blast radius
// of a change visible: unify the version comparators and these tests fail,
// and the diff is precisely the set of behaviours that moved.
//
// Do not "fix" a failing expectation here by editing the expectation. Either
// the change was intended - in which case update it deliberately and note it
// in the commit - or it was not, and the test just caught a regression.
//
// Background: luet compares versions under several incompatible regimes.
// The two that matter here:
//
//   R1 - ordering, via knqyf263/go-deb-version (Debian semantics).
//        Reached through Sort() and therefore Packages.Best().
//   R2 - selector matching, via hashicorp/go-version (SemVer semantics).
//        Reached through ValidateSelector() and therefore Expand(),
//        FindPackages(), and every candidate-filtering path.
//
// R1 decides which version is newest. R2 decides which versions are even
// eligible. They disagree, so the candidate set and the ranking of that set
// are computed under different rules.

// corpus holds version strings drawn from this repo's own tests/fixtures -
// real luet conventions, not invented ones - plus the edge cases the audit
// surfaced. Shapes represented:
//
//	plain semver            0.1, 1.0, 2.0, 5.4.2
//	date-style              20190410, 0.20191126
//	Gentoo live ebuild      9999
//	build revision          1.0+2, 2.10.1+1      (what BumpBuildVersion emits)
//	Gentoo composite        1.0.29+pre2_p20191024.1
//	Debian revision         1.0-1, 1.0-r1
//	underscore variant      1.0_1
//	epoch                   0:1.0
//	tilde prerelease        1.0~rc1
//	v-prefixed              v1.0
//	unparseable             abc, ""
var corpus = []string{
	"0.1", "1.0", "1.1", "2.0", "9999", "20190410", "0.20191126",
	"1.0+2", "2.10.1+1", "1.0.29+pre2_p20191024.1",
	"1.0-1", "1.0_1", "1.0.0", "0:1.0", "1.0~rc1", "1.0-r1", "v1.0",
}

// TestCharacterizeSortOrder pins the total order Sort produces over the corpus.
//
// Reading notes on the current output:
//   - "v1.0" sorts FIRST despite looking like 1.0: go-deb-version cannot parse a
//     v-prefix, and unparseable entries are ordered before all valid ones.
//   - "1.0~rc1" precedes "1.0": correct Debian semantics, "~" sorts before empty.
//   - "1.0" and "0:1.0" are ADJACENT and mutually equal under R1 - an explicit
//     zero epoch is a no-op. Their relative order here is input order, because
//     the sort is stable. Feed them in the other order and they swap.
//   - "1.0+2" precedes "1.0.0": "+2" is upstream-version text to Debian, not
//     the build metadata SemVer would ignore.
func TestCharacterizeSortOrder(t *testing.T) {
	want := []string{
		"v1.0",                    // unparseable, sorts first
		"0.1",                     //
		"0.20191126",              //
		"1.0~rc1",                 // tilde sorts before the bare release
		"1.0",                     //
		"0:1.0",                   // equal to "1.0" under R1; order is input order
		"1.0-1",                   //
		"1.0_1",                   // Sanitize maps "_" to "-", so equal to "1.0-1"
		"1.0-r1",                  //
		"1.0+2",                   //
		"1.0.0",                   //
		"1.0.29+pre2_p20191024.1", //
		"1.1",                     //
		"2.0",                     //
		"2.10.1+1",                //
		"9999",                    //
		"20190410",                //
	}

	got := DefaultVersioner().Sort(append([]string{}, corpus...))

	if len(got) != len(want) {
		t.Fatalf("length changed: got %d, want %d\ngot:  %q\nwant: %q", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q\nfull got:  %q\nfull want: %q",
				i, got[i], want[i], got, want)
		}
	}
}

// TestCharacterizeSelectorMatching pins ValidateSelector for cases that either
// document a real bug or guard behaviour worth keeping.
func TestCharacterizeSelectorMatching(t *testing.T) {
	for _, tt := range []struct {
		version  string
		selector string
		want     bool
		note     string
	}{
		// --- Correct behaviour, keep it that way ---
		{"1.0", ">=0", true, "the unconstrained sentinel matches"},
		{"1.0", ">=1.0", true, ""},
		{"0.1", ">=1.0", false, ""},
		{"2.0", ">1.0", true, ""},
		{"1.0", "<1.0", false, ""},

		// --- BUG: build revisions are invisible to selectors ---
		// SemVer treats everything after "+" as build metadata and excludes it
		// from precedence, but "+N" is exactly what BumpBuildVersion emits.
		// So luet's own rebuild format cannot be selected against, while Sort
		// orders the same strings apart.
		{"1.0+2", ">1.0+1", false, "BUG: should be true, +2 is newer than +1"},
		{"1.0+2", ">1.0", false, "BUG: should be true, a rebuild is newer"},
		{"1.0+1", ">=1.0+2", true, "BUG: should be false, +1 is older than +2"},

		// --- BUG: epoch versions match nothing at all ---
		// hashicorp/go-version cannot parse "0:1.0"; the parse error is
		// discarded and false returned. Sort handles the same string fine, so
		// an epoch package is rankable but never selectable - unreachable by
		// the solver despite being a legal luet version.
		{"0:1.0", ">=0", false, "BUG: should be true, >=0 matches everything"},
		{"0:1.0", ">=1.0", false, "BUG: should be true, 0:1.0 equals 1.0"},
		{"0:1.0", "<1.0", false, "BUG: matches nothing in either direction"},

		// --- BUG: Debian revisions read as SemVer prereleases ---
		// Debian: "-1" is a revision, so 1.0-1 > 1.0.
		// SemVer: "-1" is a prerelease, so 1.0-1 < 1.0.
		// The two regimes order this pair in OPPOSITE directions, and Sort
		// agrees with Debian while the filter agrees with SemVer - so the
		// version Best() considers newest is one Expand() refuses to return.
		{"1.0-1", ">=1.0", false, "BUG: R1 ranks 1.0-1 above 1.0, R2 excludes it"},
		{"1.0-1", "<1.0", true, "BUG: inverse of the above"},

		// --- The two regimes accept different languages ---
		// This is the mirror image of the epoch case above, and arguably worse
		// because it is silent in the other direction:
		//
		//   "0:1.0"  R1 parses it, R2 does not -> rankable, never selectable
		//   "v1.0"   R2 parses it, R1 does not -> selectable, never rankable
		//
		// hashicorp/go-version accepts a "v" prefix; go-deb-version rejects it.
		// So a v-prefixed package passes every selector filter and then lands in
		// Sort as an unparseable entry, ordering below every valid version - it
		// can be a candidate but can never win Best().
		{"v1.0", ">=0", true, "R2 parses the v-prefix; R1 does not (see Sort order)"},
		{"v1.0", ">=1.0", true, "so it is selectable, but unrankable"},

		// Genuinely unparseable to both: the error is discarded and false
		// returned, with no diagnostic anywhere.
		{"abc", ">=0", false, "BUG: parse failure swallowed, no diagnostic"},

		// --- BUG: the empty version matches every selector ---
		// ValidateSelector short-circuits to true on "", including for
		// constraints nothing could satisfy.
		{"", ">=99999", true, "BUG: empty version satisfies impossible constraints"},
		{"", "<0", true, "BUG: same"},

		// --- The "!" operator is registered but unreachable ---
		// Package.IsSelector gates on ContainsAny(version, "<>="), which
		// excludes "!", so "!1.0" never reaches here as a selector at all.
		// "!=1.0" works only incidentally, because it contains "=".
		{"2.0", "!=1.0", true, "works only because '=' is present"},
	} {
		t.Run(tt.version+"_"+tt.selector, func(t *testing.T) {
			got := DefaultVersioner().ValidateSelector(tt.version, tt.selector)
			if got != tt.want {
				t.Errorf("ValidateSelector(%q, %q) = %v, characterized as %v (%s)",
					tt.version, tt.selector, got, tt.want, tt.note)
			}
		})
	}
}

// TestCharacterizeRegimeDisagreement is the load-bearing test.
//
// For every ordered pair in the corpus it asks the same question two ways:
//
//	R1: does Sort rank b after a?
//	R2: does ValidateSelector say b satisfies ">a"?
//
// These should always agree. Today they disagree on 47 of 272 pairs (17.3%).
// Unifying the comparators should drive this to zero; any change to the count
// is a change in how luet decides which version is newer, and should be
// deliberate.
func TestCharacterizeRegimeDisagreement(t *testing.T) {
	const wantDisagreements = 47

	v := DefaultVersioner()

	rank := map[string]int{}
	for i, s := range v.Sort(append([]string{}, corpus...)) {
		rank[s] = i
	}

	got := 0
	for _, a := range corpus {
		for _, b := range corpus {
			if a == b {
				continue
			}
			r1Newer := rank[b] > rank[a]
			r2Matches := v.ValidateSelector(b, ">"+a)
			if r1Newer != r2Matches {
				got++
				t.Logf("disagree: %-26q vs %-26q  R1_newer=%-5v R2_gt=%v",
					b, a, r1Newer, r2Matches)
			}
		}
	}

	if got != wantDisagreements {
		t.Errorf("R1/R2 disagreements = %d, characterized as %d.\n"+
			"If this dropped, the comparators are converging - update the constant "+
			"and say so in the commit. If it rose, something regressed.",
			got, wantDisagreements)
	}
}

// TestCharacterizeSortHang documents a reachable infinite loop.
//
// go-deb-version's compare() is an unbounded `for i := 0; ; i++` that exits
// only on a non-zero difference. Two strings whose numeric segments are
// numerically equal but differ in zero-padding never produce one, so it spins
// forever at 100% CPU. Version.Compare has a reflect.DeepEqual fast path, but
// it only catches structs that are exactly equal.
//
// Both forms pass Validate(), so a tree carrying "1.0" and "1.00" of the same
// package wedges every path that calls Best(): FindPackageCandidate, getList,
// computeUpgrade, ResolveSelectors.
//
// This predates the Sort round-trip fix in #392 - the spin happens inside the
// sort comparator, before the map lookup that fix replaced. Verified against
// both the pre- and post-fix implementations.
//
// Upstream is unmaintained (pinned at v0.0.0-20190517075300, 2019), so fixing
// it means bounding the loop in a fork. The test is guarded by a timeout so it
// reports rather than hanging CI.
func TestCharacterizeSortHang(t *testing.T) {
	// The test can only prove a hang by waiting, so it costs real seconds and
	// leaks a spinning goroutine per case. Skip it under -short.
	if testing.Short() {
		t.Skip("skipping hang characterization under -short")
	}

	for _, pair := range [][]string{
		{"1.0", "1.00"},
		{"1", "01"},
		{"2.0", "2.00"},
		{"1.0-1", "1.0-01"},
	} {
		t.Run(pair[0]+"_vs_"+pair[1], func(t *testing.T) {
			done := make(chan []string, 1)
			go func() {
				// Leaks a goroutine spinning at 100% CPU. Acceptable in a test
				// process that is about to exit; it is the behaviour under test.
				defer func() { recover() }()
				done <- DefaultVersioner().Sort([]string{pair[0], pair[1]})
			}()

			select {
			case got := <-done:
				t.Errorf("Sort(%q) returned %q - the upstream hang appears fixed. "+
					"Remove this test and the fork/workaround it documents.", pair, got)
			case <-time.After(1 * time.Second):
				t.Logf("Sort(%q) did not terminate within 1s, as characterized", pair)
			}
		})
	}
}
