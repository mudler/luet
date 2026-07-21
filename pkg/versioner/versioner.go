// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package version

import (
	"errors"
	"sort"
	"strings"
	"sync"

	debversion "github.com/knqyf263/go-deb-version"
)

// Version comparison uses exactly one implementation: Debian EVR semantics via
// go-deb-version.
//
// It previously used two. Ordering (Sort, and therefore Packages.Best) was
// Debian, while selector matching (ValidateSelector, and therefore Expand and
// every candidate-filtering path) was SemVer via hashicorp/go-version. The two
// disagreed on 47 of 272 ordered pairs over a corpus of real luet versions, so
// the set of eligible candidates and the ranking of that set were computed
// under different rules. Notably:
//
//   - "+N" build revisions, which BumpBuildVersion emits, are build metadata to
//     SemVer and excluded from precedence - so luet could not select against its
//     own rebuild format even though Sort ordered the versions apart.
//   - epoch versions ("0:1.0") parse under Debian but not SemVer, making them
//     rankable but never selectable - unreachable by the solver.
//   - a Debian revision ("1.0-1") is a SemVer prerelease, so the two regimes
//     ordered it against "1.0" in opposite directions.
//
// Debian is the correct canonical choice: it is the only one of the two that
// can express luet's own conventions (epochs, revisions, "~" prereleases, and
// the "+N" rebuild counter).
const (
	// selectorEqual is deliberately the zero value, so that a selector with no
	// recognised operator ("1.0") means equality. It used to be
	// selectorGreaterThen, which meant any parse miss silently became ">".
	selectorEqual = iota
	selectorGreaterThen
	selectorLessThen
	selectorGreaterOrEqualThen
	selectorLessOrEqualThen
	selectorNotEqual
)

type packageSelector struct {
	Condition int
	Version   string
}

// Longer operators must be tried first; readPackageSelector sorts by length.
var selectors = map[string]int{
	">=": selectorGreaterOrEqualThen,
	"<=": selectorLessOrEqualThen,
	"!=": selectorNotEqual,
	"==": selectorEqual,
	">":  selectorGreaterThen,
	"<":  selectorLessThen,
	"=":  selectorEqual,
	"!":  selectorNotEqual,
}

// parseCache memoises version parsing. Versions are compared repeatedly during
// resolution - ValidateSelector alone was measured at 39% of tree-loading time,
// most of it re-parsing the same strings - and the set of distinct version
// strings in a process is bounded by the repository size.
var parseCache sync.Map // string -> parsed

type parsed struct {
	version debversion.Version
	valid   bool
}

func parseVersion(s string) (debversion.Version, bool) {
	if c, ok := parseCache.Load(s); ok {
		p := c.(parsed)
		return p.version, p.valid
	}
	v, err := debversion.NewVersion(s)
	p := parsed{version: v, valid: err == nil}
	parseCache.Store(s, p)
	return p.version, p.valid
}

func readPackageSelector(selector string) packageSelector {
	selectorType := 0
	v := ""

	k := []string{}
	for kk, _ := range selectors {
		k = append(k, kk)
	}

	sort.Slice(k, func(i, j int) bool {
		return len(k[i]) > len(k[j])
	})

	matched := false
	for _, p := range k {
		if strings.HasPrefix(selector, p) {
			selectorType = selectors[p]
			v = strings.TrimSpace(strings.TrimPrefix(selector, p))
			matched = true
			break
		}
	}

	// A selector with no operator is an exact version. This used to leave v
	// empty and fall through to a SemVer constraint parse, which happened to
	// treat a bare "1.0" as equality - so the behaviour is preserved, but it is
	// now explicit rather than incidental.
	if !matched {
		v = strings.TrimSpace(selector)
	}

	return packageSelector{
		Condition: selectorType,
		Version:   v,
	}
}

// WrappedVersioner uses different means to return unique result that is understendable by Luet
// It tries different approaches to sort, validate, and sanitize to a common versioning format
// that is understendable by the whole code
type WrappedVersioner struct{}

func DefaultVersioner() Versioner {
	return &WrappedVersioner{}
}

func (w *WrappedVersioner) Validate(version string) error {
	if !debversion.Valid(version) {
		return errors.New("invalid version")
	}
	return nil
}

func (w *WrappedVersioner) ValidateSelector(vv string, selector string) bool {
	if vv == "" {
		return true
	}
	vv = w.Sanitize(vv)
	selector = w.Sanitize(selector)

	sel := readPackageSelector(selector)

	// Sanitize again after splitting off the operator: normalisation that keys
	// on the first character - the "v" prefix - cannot fire while the operator
	// is still attached, so ">v1.0" would otherwise reach the parser as "v1.0".
	sel.Version = w.Sanitize(sel.Version)

	v, okVersion := parseVersion(vv)
	s, okSelector := parseVersion(sel.Version)

	if !okVersion || !okSelector {
		// Ordering against something that is not a version is meaningless, but
		// equality still has an obvious answer. Previously an unparseable input
		// fell through to a SemVer check that also failed, and the error was
		// discarded - so every selector, including ">=0", silently returned
		// false.
		switch sel.Condition {
		case selectorEqual:
			return vv == sel.Version
		case selectorNotEqual:
			return vv != sel.Version
		}
		return false
	}

	// Compare returns an unnormalised magnitude (-299, 263, ...), not -1/0/1.
	// Test the sign, never equality against 1.
	cmp := v.Compare(s)

	switch sel.Condition {
	case selectorEqual:
		return cmp == 0
	case selectorNotEqual:
		return cmp != 0
	case selectorGreaterOrEqualThen:
		return cmp >= 0
	case selectorLessOrEqualThen:
		return cmp <= 0
	case selectorLessThen:
		return cmp < 0
	case selectorGreaterThen:
		return cmp > 0
	}

	return false
}

func (w *WrappedVersioner) Sanitize(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "_", "-"))
	s = stripVersionPrefix(s)
	return normalizeNumericSegments(s)
}

// stripVersionPrefix drops a leading "v" from "v1.0".
//
// go-deb-version rejects the form while SemVer accepted it, which made
// v-prefixed packages selectable but unrankable - they passed every selector
// filter and then sorted below every valid version, so they could be candidates
// but could never win Best(). Normalising here makes the two agree.
func stripVersionPrefix(s string) string {
	if len(s) > 1 && (s[0] == 'v' || s[0] == 'V') && s[1] >= '0' && s[1] <= '9' {
		return s[1:]
	}
	return s
}

// normalizeNumericSegments strips leading zeros from runs of digits, so "1.00"
// becomes "1.0" and "01" becomes "1".
//
// This is required for termination, not just tidiness. go-deb-version's
// compare() is an unbounded `for i := 0; ; i++` that exits only on a non-zero
// difference; two strings whose numeric segments are numerically equal but
// differ in zero-padding never produce one, so it spins forever. Version.Compare
// has a reflect.DeepEqual fast path, but it only catches exactly-equal structs.
//
// Sort could already reach that hang. Moving selector matching onto the same
// library would have widened it to every candidate filter, so the padding is
// normalised away before any comparison happens. Numerically the two forms are
// equal under Debian semantics anyway, so this changes no ordering.
//
// Upstream is unmaintained (pinned at a 2019 pseudo-version); fixing it there
// would mean carrying a fork.
func normalizeNumericSegments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] < '0' || s[i] > '9' {
			b.WriteByte(s[i])
			i++
			continue
		}

		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}

		// Keep at least one digit, so "00" collapses to "0" rather than "".
		digits := s[start:i]
		trimmed := strings.TrimLeft(digits, "0")
		if trimmed == "" {
			trimmed = "0"
		}
		b.WriteString(trimmed)
	}

	return b.String()
}

func (w *WrappedVersioner) Sort(toSort []string) []string {
	if len(toSort) == 0 {
		return toSort
	}
	// Each original string is carried alongside its parsed form and the pairs
	// are sorted together.
	//
	// This used to key a map on the sanitized input, sort the parsed versions,
	// then recover the originals with versionsMap[v.String()]. That is unsound:
	// String() re-renders rather than echoing the input, so a lookup misses
	// whenever the two differ - it drops an explicit zero epoch ("0:1.0" ->
	// "1.0"), and yields "" for anything that failed to parse, because the error
	// was discarded and the zero Version kept. A miss returned "", silently
	// substituting an empty string for a real version. Callers such as
	// Packages.Best then looked the result up and got nil, which several call
	// sites dereference. Sanitize also maps "_" to "-", so "1.0-1" and "1.0_1"
	// collided on the same key and one overwrote the other.
	type parsedVersion struct {
		original string
		parsed   debversion.Version
		valid    bool
	}

	versions := make([]parsedVersion, len(toSort))
	for i, v := range toSort {
		p, err := debversion.NewVersion(w.Sanitize(v))
		versions[i] = parsedVersion{original: v, parsed: p, valid: err == nil}
	}

	// Versions that could not be parsed sort before every valid one, keeping
	// their relative input order. They previously ended up first as a side
	// effect of collapsing to ""; this makes it a deliberate choice. Stable
	// sorting keeps the output deterministic for versions that compare equal.
	sort.SliceStable(versions, func(i, j int) bool {
		a, b := versions[i], versions[j]
		switch {
		case !a.valid && !b.valid:
			return false
		case !a.valid:
			return true
		case !b.valid:
			return false
		default:
			return a.parsed.LessThan(b.parsed)
		}
	})

	result := make([]string, 0, len(versions))
	for _, v := range versions {
		result = append(result, v.original)
	}
	return result
}
