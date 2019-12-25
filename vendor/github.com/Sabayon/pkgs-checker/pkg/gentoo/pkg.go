/*

Copyright (C) 2017-2019  Daniele Rondina <geaaru@sabayonlinux.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/
package gentoo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	version "github.com/hashicorp/go-version"
)

// ----------------------------------
// Code to move and merge inside luet project
// ----------------------------------

// Package condition
type PackageCond int

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

type GentooPackage struct {
	Name          string `json:"name",omitempty"`
	Category      string `json:"category",omitempty"`
	Version       string `json:"version",omitempty"`
	VersionSuffix string `json:"version_suffix",omitempty"`
	Slot          string `json:"slot",omitempty"`
	Condition     PackageCond
	Repository    string   `json:"repository",omitempty"`
	UseFlags      []string `json:"use_flags",omitempty"`
}

func (p *GentooPackage) String() string {
	// TODO
	opt := ""
	if p.Version != "" {
		opt = "-"
	}
	return fmt.Sprintf("%s/%s%s%s%s",
		p.Category, p.Name, opt,
		p.Version, p.VersionSuffix)
}

func (p PackageCond) String() (ans string) {
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
		ans = "="
	} else if p == PkgCondNot {
		ans = "!"
	} else if p == PkgCondAnyRevision {
		ans = "~"
	} else if p == PkgCondMatchVersion {
		ans = "=*"
	}

	return ans
}

func sanitizeVersion(v string) string {
	// https://devmanual.gentoo.org/ebuild-writing/file-format/index.html
	ans := strings.ReplaceAll(v, "_alpha", "-alpha")
	ans = strings.ReplaceAll(ans, "_beta", "-beta")
	ans = strings.ReplaceAll(ans, "_pre", "-pre")
	ans = strings.ReplaceAll(ans, "_rc", "-rc")
	ans = strings.ReplaceAll(ans, "_p", "-p")

	return ans
}

func (p *GentooPackage) OfPackage(i *GentooPackage) (ans bool) {
	if p.Category == i.Category && p.Name == i.Name {
		ans = true
	} else {
		ans = false
	}
	return
}

func (p *GentooPackage) GetPackageName() (ans string) {
	ans = fmt.Sprintf("%s/%s", p.Category, p.Name)
	return
}

func (p *GentooPackage) GetP() string {
	return fmt.Sprintf("%s-%s", p.Name, p.GetPV())
}

func (p *GentooPackage) GetPN() string {
	return p.Name
}

func (p *GentooPackage) GetPV() string {
	return fmt.Sprintf("%s", p.Version)
}

func (p *GentooPackage) GetPVR() (ans string) {
	if p.VersionSuffix != "" {
		ans = fmt.Sprintf("%s%s", p.Version, p.VersionSuffix)
	} else {
		ans = p.GetPV()
	}
	return
}

func (p *GentooPackage) GetPF() string {
	return fmt.Sprintf("%s-%s", p.GetPN(), p.GetPVR())
}

func (p *GentooPackage) Admit(i *GentooPackage) (bool, error) {
	var ans bool = false
	var v1 *version.Version = nil
	var v2 *version.Version = nil
	var err error

	if p.Category != i.Category {
		return false, errors.New(
			fmt.Sprintf("Wrong category for package %s", i.Name))
	}

	if p.Name != i.Name {
		return false, errors.New(
			fmt.Sprintf("Wrong name for package %s", i.Name))
	}

	if p.Version != "" {
		v1, err = version.NewVersion(sanitizeVersion(p.Version))
		if err != nil {
			return false, err
		}
	}
	if i.Version != "" {
		v2, err = version.NewVersion(sanitizeVersion(i.Version))
		if err != nil {
			return false, err
		}
	}

	// If package doesn't define version admit all versions of the package.
	if p.Version == "" {
		ans = true
	} else {
		if p.Condition == PkgCondInvalid || p.Condition == PkgCondEqual {
			// case 1: source-pkg-1.0 and dest-pkg-1.0 or dest-pkg without version
			if i.Version != "" && i.Version == p.Version && p.VersionSuffix == i.VersionSuffix {
				ans = true
			}
		} else if p.Condition == PkgCondAnyRevision {
			if v1 != nil && v2 != nil {
				ans = v1.Equal(v2)
			}
		} else if p.Condition == PkgCondMatchVersion {
			// TODO: case of 7.3* where 7.30 is accepted.
			if v1 != nil && v2 != nil {
				segments := v1.Segments()
				n := strings.Count(p.Version, ".")
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
				constraints, err := version.NewConstraint(
					fmt.Sprintf(">= %s, < %s", p.Version, nextVersion),
				)
				if err != nil {
					return false, err
				}
				ans = constraints.Check(v2)
			}
		} else if v1 != nil && v2 != nil {

			switch p.Condition {
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

// return category, package, version, slot, condition
func ParsePackageStr(pkg string) (*GentooPackage, error) {
	if pkg == "" {
		return nil, errors.New("Invalid package string")
	}

	ans := GentooPackage{
		Slot:      "0",
		Condition: PkgCondInvalid,
	}

	// Check if pkg string contains inline use flags
	regexUses := regexp.MustCompile(
		"\\[([a-z]*[-]*[0-9]*[,]*)+\\]*$",
	)
	mUses := regexUses.FindAllString(pkg, -1)
	if len(mUses) > 0 {
		ans.UseFlags = strings.Split(
			pkg[len(pkg)-len(mUses[0])+1:len(pkg)-1],
			",",
		)
		pkg = pkg[:len(pkg)-len(mUses[0])]
	}

	if strings.HasPrefix(pkg, ">=") {
		pkg = pkg[2:]
		ans.Condition = PkgCondGreaterEqual
	} else if strings.HasPrefix(pkg, ">") {
		pkg = pkg[1:]
		ans.Condition = PkgCondGreater
	} else if strings.HasPrefix(pkg, "<=") {
		pkg = pkg[2:]
		ans.Condition = PkgCondLessEqual
	} else if strings.HasPrefix(pkg, "<") {
		pkg = pkg[1:]
		ans.Condition = PkgCondLess
	} else if strings.HasPrefix(pkg, "=") {
		pkg = pkg[1:]
		if strings.HasSuffix(pkg, "*") {
			ans.Condition = PkgCondMatchVersion
			pkg = pkg[0 : len(pkg)-1]
		} else {
			ans.Condition = PkgCondEqual
		}
	} else if strings.HasPrefix(pkg, "~") {
		pkg = pkg[1:]
		ans.Condition = PkgCondAnyRevision
	} else if strings.HasPrefix(pkg, "!") {
		pkg = pkg[1:]
		ans.Condition = PkgCondNot
	}

	words := strings.Split(pkg, "/")
	if len(words) != 2 {
		return nil, errors.New(fmt.Sprintf("Invalid package string %s", pkg))
	}
	ans.Category = words[0]
	pkgname := words[1]

	// Check if has repository
	if strings.Contains(pkgname, "::") {
		words = strings.Split(pkgname, "::")
		ans.Repository = words[1]
		pkgname = words[0]
	}

	// Check if has slot
	if strings.Contains(pkgname, ":") {
		words = strings.Split(pkgname, ":")
		ans.Slot = words[1]
		pkgname = words[0]
	}

	regexPkg := regexp.MustCompile(
		fmt.Sprintf("[-](%s|%s|%s|%s|%s|%s)((%s|%s|%s|%s|%s|%s|%s)+)*$",
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
	matches := regexPkg.FindAllString(pkgname, -1)

	// NOTE: Now suffix comples like _alpha_rc1 are not supported.

	if len(matches) > 0 {
		// Check if there patch
		if strings.Contains(matches[0], "_p") {
			ans.Version = matches[0][1:strings.Index(matches[0], "_p")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_p"):]
		} else if strings.Contains(matches[0], "_rc") {
			ans.Version = matches[0][1:strings.Index(matches[0], "_rc")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_rc"):]
		} else if strings.Contains(matches[0], "_alpha") {
			ans.Version = matches[0][1:strings.Index(matches[0], "_alpha")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_alpha"):]
		} else if strings.Contains(matches[0], "_beta") {
			ans.Version = matches[0][1:strings.Index(matches[0], "_beta")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "_beta"):]
		} else if strings.Contains(matches[0], "-r") {
			ans.Version = matches[0][1:strings.Index(matches[0], "-r")]
			ans.VersionSuffix = matches[0][strings.Index(matches[0], "-r"):]
		} else {
			ans.Version = matches[0][1:]
		}
		ans.Name = pkgname[0 : len(pkgname)-len(ans.Version)-1-len(ans.VersionSuffix)]
	} else {
		ans.Name = pkgname
	}

	// Set condition if there isn't a prefix but only a version
	if ans.Condition == PkgCondInvalid && ans.Version != "" {
		ans.Condition = PkgCondEqual
	}

	return &ans, nil
}
