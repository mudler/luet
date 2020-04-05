// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>,
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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
	"fmt"
	"regexp"
	"strings"

	semver "github.com/hashicorp/go-version"
)

// Package Selector Condition
type PkgSelectorCondition int

type PkgVersionSelector struct {
	Version       string
	VersionSuffix string
	Condition     PkgSelectorCondition
	// TODO: Integrate support for multiple repository
}

const (
	PkgCondInvalid = 0
	// >
	PkgCondGreater = 1
	// >=
	PkgCondGreaterEqual = 2
	// <
	PkgCondLess = 3
	// <=
	PkgCondLessEqual = 4
	// =
	PkgCondEqual = 5
	// !
	PkgCondNot = 6
	// ~
	PkgCondAnyRevision = 7
	// =<pkg>*
	PkgCondMatchVersion = 8
)

func PkgSelectorConditionFromInt(c int) (ans PkgSelectorCondition) {
	if c == PkgCondGreater {
		ans = PkgCondGreater
	} else if c == PkgCondGreaterEqual {
		ans = PkgCondGreaterEqual
	} else if c == PkgCondLess {
		ans = PkgCondLess
	} else if c == PkgCondLessEqual {
		ans = PkgCondLessEqual
	} else if c == PkgCondNot {
		ans = PkgCondNot
	} else if c == PkgCondAnyRevision {
		ans = PkgCondAnyRevision
	} else if c == PkgCondMatchVersion {
		ans = PkgCondMatchVersion
	} else {
		ans = PkgCondInvalid
	}
	return
}

func (p PkgSelectorCondition) String() (ans string) {
	if p == PkgCondInvalid {
		ans = ""
	} else if p == PkgCondGreater {
		ans = ">"
	} else if p == PkgCondGreaterEqual {
		ans = ">="
	} else if p == PkgCondLess {
		ans = "<"
	} else if p == PkgCondLessEqual {
		ans = "<="
	} else if p == PkgCondEqual {
		// To permit correct matching on database
		// we currently use directly package version without =
		ans = ""
	} else if p == PkgCondNot {
		ans = "!"
	} else if p == PkgCondAnyRevision {
		ans = "~"
	} else if p == PkgCondMatchVersion {
		ans = "=*"
	}
	return
}

func (p PkgSelectorCondition) Int() (ans int) {
	if p == PkgCondInvalid {
		ans = PkgCondInvalid
	} else if p == PkgCondGreater {
		ans = PkgCondGreater
	} else if p == PkgCondGreaterEqual {
		ans = PkgCondGreaterEqual
	} else if p == PkgCondLess {
		ans = PkgCondLess
	} else if p == PkgCondLessEqual {
		ans = PkgCondLessEqual
	} else if p == PkgCondEqual {
		// To permit correct matching on database
		// we currently use directly package version without =
		ans = PkgCondEqual
	} else if p == PkgCondNot {
		ans = PkgCondNot
	} else if p == PkgCondAnyRevision {
		ans = PkgCondAnyRevision
	} else if p == PkgCondMatchVersion {
		ans = PkgCondMatchVersion
	}
	return
}

func ParseVersion(v string) (PkgVersionSelector, error) {
	var ans PkgVersionSelector = PkgVersionSelector{
		Version:       "",
		VersionSuffix: "",
		Condition:     PkgCondInvalid,
	}

	if strings.HasPrefix(v, ">=") {
		v = v[2:]
		ans.Condition = PkgCondGreaterEqual
	} else if strings.HasPrefix(v, ">") {
		v = v[1:]
		ans.Condition = PkgCondGreater
	} else if strings.HasPrefix(v, "<=") {
		v = v[2:]
		ans.Condition = PkgCondLessEqual
	} else if strings.HasPrefix(v, "<") {
		v = v[1:]
		ans.Condition = PkgCondLess
	} else if strings.HasPrefix(v, "=") {
		v = v[1:]
		if strings.HasSuffix(v, "*") {
			ans.Condition = PkgCondMatchVersion
			v = v[0 : len(v)-1]
		} else {
			ans.Condition = PkgCondEqual
		}
	} else if strings.HasPrefix(v, "~") {
		v = v[1:]
		ans.Condition = PkgCondAnyRevision
	} else if strings.HasPrefix(v, "!") {
		v = v[1:]
		ans.Condition = PkgCondNot
	}

	// Check if build number is present
	buildIdx := strings.LastIndex(v, "+")
	buildVersion := ""
	if buildIdx > 0 {
		// <pre-release> ::= <dot-separated pre-release identifiers>
		//
		// <dot-separated pre-release identifiers> ::=
		//      <pre-release identifier> | <pre-release identifier> "."
		//      <dot-separated pre-release identifiers>
		//
		// <build> ::= <dot-separated build identifiers>
		//
		// <dot-separated build identifiers> ::= <build identifier>
		//      | <build identifier> "." <dot-separated build identifiers>
		//
		// <pre-release identifier> ::= <alphanumeric identifier>
		//                            | <numeric identifier>
		//
		// <build identifier> ::= <alphanumeric identifier>
		//      | <digits>
		//
		// <alphanumeric identifier> ::= <non-digit>
		//      | <non-digit> <identifier characters>
		//      | <identifier characters> <non-digit>
		//      | <identifier characters> <non-digit> <identifier characters>
		buildVersion = v[buildIdx:]
		v = v[0:buildIdx]
	}

	regexPkg := regexp.MustCompile(
		fmt.Sprintf("(%s|%s|%s|%s|%s|%s)((%s|%s|%s|%s|%s|%s|%s)+)*$",
			// Version regex
			// 1.1
			"[0-9]+[.][0-9]+[a-z]*",
			// 1
			"[0-9]+[a-z]*",
			// 1.1.1
			"[0-9]+[.][0-9]+[.][0-9]+[a-z]*",
			// 1.1.1.1
			"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
			// 1.1.1.1.1
			"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
			// 1.1.1.1.1.1
			"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
			// suffix
			"-r[0-9]+",
			"_p[0-9]+",
			"_pre[0-9]*",
			"_rc[0-9]+",
			// handle also rc without number
			"_rc",
			"_alpha",
			"_beta",
		),
	)
	matches := regexPkg.FindAllString(v, -1)

	if len(matches) > 0 {
		// Check if there patch
		if strings.Contains(matches[0], "_p") {
			ans.Version = matches[0][0:strings.Index(matches[0], "_p")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_p"):]
		} else if strings.Contains(matches[0], "_rc") {
			ans.Version = matches[0][0:strings.Index(matches[0], "_rc")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_rc"):]
		} else if strings.Contains(matches[0], "_alpha") {
			ans.Version = matches[0][0:strings.Index(matches[0], "_alpha")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_alpha"):]
		} else if strings.Contains(matches[0], "_beta") {
			ans.Version = matches[0][0:strings.Index(matches[0], "_beta")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_beta"):]
		} else if strings.Contains(matches[0], "-r") {
			ans.Version = matches[0][0:strings.Index(matches[0], "-r")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "-r"):]
		} else {
			ans.Version = matches[0]
		}
	}

	// Set condition if there isn't a prefix but only a version
	if ans.Condition == PkgCondInvalid && ans.Version != "" {
		ans.Condition = PkgCondEqual
	}

	ans.Version += buildVersion

	// NOTE: Now suffix complex like _alpha_rc1 are not supported.
	return ans, nil
}

func PackageAdmit(selector, i PkgVersionSelector) (bool, error) {
	var v1 *semver.Version = nil
	var v2 *semver.Version = nil
	var ans bool
	var err error
	var sanitizedSelectorVersion, sanitizedIVersion string

	if selector.Version != "" {
		// TODO: This is temporary!. I promise it.
		sanitizedSelectorVersion = strings.ReplaceAll(selector.Version, "_", "-")

		v1, err = semver.NewVersion(sanitizedSelectorVersion)
		if err != nil {
			return false, err
		}
	}
	if i.Version != "" {
		sanitizedIVersion = strings.ReplaceAll(i.Version, "_", "-")
		v2, err = semver.NewVersion(sanitizedIVersion)
		if err != nil {
			return false, err
		}
	} else {
		// If version is not defined match always package
		ans = true
	}

	// If package doesn't define version admit all versions of the package.
	if selector.Version == "" {
		ans = true
	} else {
		if selector.Condition == PkgCondInvalid || selector.Condition == PkgCondEqual {
			// case 1: source-pkg-1.0 and dest-pkg-1.0 or dest-pkg without version
			if i.Version != "" && i.Version == selector.Version && selector.VersionSuffix == i.VersionSuffix {
				ans = true
			}
		} else if selector.Condition == PkgCondAnyRevision {
			if v1 != nil && v2 != nil {
				ans = v1.Equal(v2)
			}
		} else if selector.Condition == PkgCondMatchVersion {
			// TODO: case of 7.3* where 7.30 is accepted.
			if v1 != nil && v2 != nil {
				segments := v1.Segments()
				n := strings.Count(sanitizedIVersion, ".")
				switch n {
				case 0:
					segments[0]++
				case 1:
					segments[1]++
				case 2:
					segments[2]++
				default:
					segments[len(segments)-1]++
				}
				nextVersion := strings.Trim(strings.Replace(fmt.Sprint(segments), " ", ".", -1), "[]")
				constraints, err := semver.NewConstraint(
					fmt.Sprintf(">= %s, < %s", sanitizedSelectorVersion, nextVersion),
				)
				if err != nil {
					return false, err
				}
				ans = constraints.Check(v2)
			}
		} else if v1 != nil && v2 != nil {

			// TODO: Integrate check of version suffix
			switch selector.Condition {
			case PkgCondGreaterEqual:
				ans = v2.GreaterThanOrEqual(v1)
			case PkgCondLessEqual:
				ans = v2.LessThanOrEqual(v1)
			case PkgCondGreater:
				ans = v2.GreaterThan(v1)
			case PkgCondLess:
				ans = v2.LessThan(v1)
			case PkgCondNot:
				ans = !v2.Equal(v1)
			}
		}
	}

	return ans, nil
}
