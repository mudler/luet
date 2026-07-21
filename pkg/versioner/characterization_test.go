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
// These pin the observable behaviour of version comparison. Their purpose is
// to make the blast radius of a change visible: touch the comparator and the
// diff here is precisely the set of behaviours that moved.
//
// Do not "fix" a failing expectation by editing the expectation. Either the
// change was intended - in which case update it deliberately and say so in the
// commit, as was done when the comparators were unified - or it was not, and
// the test just caught a regression.
//
// Background. luet used to compare versions under two incompatible regimes:
//
//   R1 - ordering, via knqyf263/go-deb-version (Debian semantics).
//        Reached through Sort() and therefore Packages.Best().
//   R2 - selector matching, via hashicorp/go-version (SemVer semantics).
//        Reached through ValidateSelector() and therefore Expand(),
//        FindPackages(), and every candidate-filtering path.
//
// R1 decided which version was newest; R2 decided which versions were even
// eligible. They disagreed on 47 of 272 ordered pairs over this corpus, so the
// candidate set and the ranking of that set were computed under different
// rules. Both now use Debian semantics and the disagreement count is 0; the
// comments below record what each case did before, since that is what a real
// repository may have been relying on.

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
//   - "v1.0" sorts adjacent to "1.0" and compares equal to it: the "v" prefix is
//     normalised away before parsing. It previously sorted FIRST, among the
//     unparseable entries, because go-deb-version rejects the form.
//   - "1.0~rc1" precedes "1.0": correct Debian semantics, "~" sorts before empty.
//   - "1.0", "0:1.0" and "v1.0" are ADJACENT and mutually equal - an explicit
//     zero epoch is a no-op, and the "v" prefix is stripped. Their relative
//     order is input order, because the sort is stable. Feed them in a
//     different order and they swap; that is expected for equal elements.
//   - "1.0+2" precedes "1.0.0": "+2" is upstream-version text to Debian, not
//     the build metadata SemVer would ignore.
func TestCharacterizeSortOrder(t *testing.T) {
	want := []string{
		"0.1",                     //
		"0.20191126",              //
		"1.0~rc1",                 // tilde sorts before the bare release
		"1.0",                     //
		"0:1.0",                   // == "1.0", an explicit zero epoch is a no-op
		"v1.0",                    // == "1.0", the "v" prefix is normalised away
		"1.0-1",                   //
		"1.0_1",                   // == "1.0-1", Sanitize maps "_" to "-"
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

		// --- Build revisions now order (WAS: invisible to selectors) ---
		// SemVer treats everything after "+" as build metadata and excludes it
		// from precedence, but "+N" is exactly what BumpBuildVersion emits - so
		// luet could not select against its own rebuild format, while Sort
		// ordered the same strings apart. All three of these returned the
		// opposite answer before unification.
		//
		// This is the change most likely to be user-visible: packages that
		// previously tied on "+N" now resolve to a specific rebuild.
		{"1.0+2", ">1.0+1", true, "a rebuild is newer than the one before it"},
		{"1.0+2", ">1.0", true, "a rebuild is newer than the bare version"},
		{"1.0+1", ">=1.0+2", false, "an older rebuild does not satisfy a newer floor"},

		// --- Epoch versions are selectable (WAS: matched nothing) ---
		// hashicorp/go-version cannot parse "0:1.0"; the parse error was
		// discarded and false returned, for every selector including ">=0".
		// Sort handled the same string fine, so an epoch package was rankable
		// but never selectable - unreachable by the solver despite being a
		// legal luet version. Such packages become reachable now.
		{"0:1.0", ">=0", true, "epoch versions are selectable"},
		{"0:1.0", ">=1.0", true, "an explicit zero epoch equals the bare version"},
		{"0:1.0", "<1.0", false, "equal, so not strictly less"},

		// --- Debian revisions (WAS: read as SemVer prereleases) ---
		// Debian: "-1" is a revision, so 1.0-1 > 1.0.
		// SemVer: "-1" is a prerelease, so 1.0-1 < 1.0.
		// The regimes ordered this pair in OPPOSITE directions - Sort agreed
		// with Debian, the filter with SemVer - so the version Best() considered
		// newest was one Expand() refused to return. Debian semantics win.
		{"1.0-1", ">=1.0", true, "a Debian revision is newer, and both regimes agree"},
		{"1.0-1", "<1.0", false, "inverse of the above"},

		// --- v-prefixed versions (WAS: selectable but unrankable) ---
		// The two regimes used to accept different languages, in opposite
		// directions:
		//
		//   "0:1.0"  R1 parsed it, R2 did not -> rankable, never selectable
		//   "v1.0"   R2 parsed it, R1 did not -> selectable, never rankable
		//
		// A v-prefixed package passed every selector filter, then landed in Sort
		// as an unparseable entry ordering below every valid version: it could
		// be a candidate but could never win Best(). Sanitize now strips the
		// prefix, so both regimes see the same version.
		{"v1.0", ">=0", true, "the v-prefix is normalised: selectable and rankable"},
		{"v1.0", ">=1.0", true, "and it compares equal to 1.0"},

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
// These must agree. Before the comparators were unified they disagreed on 47
// of 272 ordered pairs (17.3%); it is now 0. A non-zero count means ordering
// and eligibility have drifted apart again, which is the defect class this
// whole exercise exists to prevent.
//
// Pairs that compare EQUAL are skipped. Sort is stable, so equal elements keep
// their input order and "ranked after" carries no meaning for them - including
// such a pair would measure the corpus's declaration order, not the
// comparator. Ties are identified with R2's own equality operator rather than
// by reaching into R1.
func TestCharacterizeRegimeDisagreement(t *testing.T) {
	const wantDisagreements = 0

	v := DefaultVersioner()

	rank := map[string]int{}
	for i, s := range v.Sort(append([]string{}, corpus...)) {
		rank[s] = i
	}

	got := 0
	for _, a := range corpus {
		for _, b := range corpus {
			if a == b || v.ValidateSelector(b, "="+a) {
				continue // same string, or an equal-comparing tie
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
		t.Errorf("R1/R2 disagreements = %d, want %d.\n"+
			"Ordering (Sort/Best) and eligibility (ValidateSelector/Expand) have "+
			"drifted apart. Both must use the same comparator.",
			got, wantDisagreements)
	}
}

// TestCharacterizeZeroPaddingTerminates guards a fix for a reachable hang.
//
// go-deb-version's compare() is an unbounded `for i := 0; ; i++` that exits
// only on a non-zero difference. Two strings whose numeric segments are
// numerically equal but differ in zero-padding never produce one, so it spins
// forever at 100% CPU. Version.Compare has a reflect.DeepEqual fast path, but
// it only catches exactly-equal structs.
//
// Both forms pass Validate(), so a tree carrying "1.0" and "1.00" of the same
// package used to wedge every path that calls Best(): FindPackageCandidate,
// getList, computeUpgrade, ResolveSelectors. The hang predates the Sort
// round-trip fix in #392 - the spin is inside the sort comparator, before the
// map lookup that fix replaced.
//
// Sanitize now strips leading zeros from digit runs, so the two forms are
// string-identical by the time they reach the parser. Numerically they are
// equal under Debian semantics anyway, so no ordering changed.
//
// Upstream is unmaintained (pinned at a 2019 pseudo-version), so this is
// normalisation on our side rather than a fix there. If the padding
// normalisation is ever removed, these cases hang again.
func TestCharacterizeZeroPaddingTerminates(t *testing.T) {
	for _, pair := range [][]string{
		{"1.0", "1.00"},
		{"1", "01"},
		{"2.0", "2.00"},
		{"1.0-1", "1.0-01"},
		{"0", "00"},
		{"1.0", "1.000"},
	} {
		t.Run(pair[0]+"_vs_"+pair[1], func(t *testing.T) {
			done := make(chan []string, 1)
			go func() {
				defer func() { recover() }()
				done <- DefaultVersioner().Sort([]string{pair[0], pair[1]})
			}()

			select {
			case got := <-done:
				if len(got) != 2 {
					t.Fatalf("Sort(%q) returned %q, want both elements", pair, got)
				}
				// Zero-padded forms are numerically equal, so a stable sort must
				// leave them in input order.
				if got[0] != pair[0] || got[1] != pair[1] {
					t.Errorf("Sort(%q) = %q, want input order preserved for equal versions",
						pair, got)
				}
				if !DefaultVersioner().ValidateSelector(pair[1], "="+pair[0]) {
					t.Errorf("ValidateSelector(%q, \"=%s\") = false, want true - "+
						"zero-padded forms are numerically equal", pair[1], pair[0])
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("Sort(%q) did not terminate within 5s - the zero-padding "+
					"normalisation in Sanitize has regressed, and the upstream "+
					"comparator is spinning", pair)
			}
		})
	}
}
