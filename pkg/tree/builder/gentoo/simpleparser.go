// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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

package gentoo

// NOTE: Look here as an example of the builder definition executor
// https://gist.github.com/adnaan/6ca68c7985c6f851def3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/mudler/luet/pkg/logger"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	pkg "github.com/mudler/luet/pkg/package"
	"mvdan.cc/sh/expand"
	"mvdan.cc/sh/shell"
	"mvdan.cc/sh/syntax"
)

// SimpleEbuildParser ignores USE flags and generates just 1-1 package
type SimpleEbuildParser struct {
	World pkg.PackageDatabase
}

type GentooDependency struct {
	Use          string
	UseCondition _gentoo.PackageCond
	SubDeps      []*_gentoo.GentooPackage
	Dep          *_gentoo.GentooPackage
}

type GentooRDEPEND struct {
	Dependencies []*GentooDependency
}

func NewGentooDependency(pkg, use string) (*GentooDependency, error) {
	var err error
	ans := &GentooDependency{
		Use:     use,
		SubDeps: make([]*_gentoo.GentooPackage, 0),
	}

	if strings.HasPrefix(use, "!") {
		ans.Use = ans.Use[1:]
		ans.UseCondition = _gentoo.PkgCondNot
	}

	if pkg != "" {
		ans.Dep, err = _gentoo.ParsePackageStr(pkg)
		if err != nil {
			return nil, err
		}
	}

	return ans, nil
}

func (d *GentooDependency) AddSubDependency(pkg string) (*_gentoo.GentooPackage, error) {
	ans, err := _gentoo.ParsePackageStr(pkg)
	if err != nil {
		return nil, err
	}

	d.SubDeps = append(d.SubDeps, ans)

	return ans, nil
}

func ParseRDEPEND(rdepend string) (*GentooRDEPEND, error) {
	var lastdep *GentooDependency
	var pendingDep = false
	var err error

	ans := &GentooRDEPEND{
		Dependencies: make([]*GentooDependency, 0),
	}

	if rdepend != "" {
		rdepends := strings.Split(rdepend, "\n")
		for _, rr := range rdepends {
			rr = strings.TrimSpace(rr)
			if rr == "" {
				continue
			}

			if strings.Index(rr, "?") > 0 {
				// use flag present

				dep, err := NewGentooDependency("", rr[:strings.Index(rr, "?")])
				if err != nil {
					return nil, err
				}
				if strings.Index(rr, ")") < 0 {
					pendingDep = true
					lastdep = dep
				}

				ans.Dependencies = append(ans.Dependencies, dep)

				fields := strings.Split(rr[strings.Index(rr, "?")+1:], " ")
				for _, f := range fields {
					f = strings.TrimSpace(f)
					if f == ")" || f == "(" || f == "" {
						continue
					}

					_, err = dep.AddSubDependency(f)
					if err != nil {
						return nil, err
					}
				}

			} else if pendingDep {
				fields := strings.Split(rr, " ")
				for _, f := range fields {
					f = strings.TrimSpace(f)
					if f == ")" || f == "(" || f == "" {
						continue
					}
					_, err = lastdep.AddSubDependency(f)
					if err != nil {
						return nil, err
					}
				}

				if strings.Index(rr, ")") >= 0 {
					pendingDep = false
					lastdep = nil
				}

			} else {
				dep, err := NewGentooDependency(rr, "")
				if err != nil {
					return nil, err
				}
				ans.Dependencies = append(ans.Dependencies, dep)
			}

		}

	}

	return ans, nil
}

func SourceFile(ctx context.Context, path string) (map[string]expand.Variable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open: %v", err)
	}
	defer f.Close()
	file, err := syntax.NewParser(syntax.StopAt("src")).Parse(f, path)
	if err != nil {
		return nil, fmt.Errorf("could not parse: %v", err)
	}
	return shell.SourceNode(ctx, file)
}

// ScanEbuild returns a list of packages (always one with SimpleEbuildParser) decoded from an ebuild.
func (ep *SimpleEbuildParser) ScanEbuild(path string, tree pkg.Tree) ([]pkg.Package, error) {
	Debug("Starting parsing of ebuild", path)

	pkgstr := filepath.Base(path)
	paths := strings.Split(filepath.Dir(path), "/")
	pkgstr = paths[len(paths)-2] + "/" + strings.Replace(pkgstr, ".ebuild", "", -1)

	gp, err := _gentoo.ParsePackageStr(pkgstr)
	if err != nil {
		return []pkg.Package{}, errors.New("Error on parsing package string")
	}

	pack := &pkg.DefaultPackage{
		Name:     gp.Name,
		Version:  fmt.Sprintf("%s%s", gp.Version, gp.VersionSuffix),
		Category: gp.Category,
	}

	// Adding a timeout of 60secs, as with some bash files it can hang indefinetly
	timeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	vars, err := SourceFile(timeout, path)
	if err != nil {
		return []pkg.Package{pack}, nil
		//	return []pkg.Package{}, err
	}

	// TODO: Handle this a bit better

	rdepend, ok := vars["RDEPEND"]
	if ok {
		gRDEPEND, err := ParseRDEPEND(rdepend.String())
		if err != nil {
			return []pkg.Package{pack}, nil
			//	return []pkg.Package{}, err
		}

		pack.PackageConflicts = []*pkg.DefaultPackage{}
		pack.PackageRequires = []*pkg.DefaultPackage{}
		for _, d := range gRDEPEND.Dependencies {

			// TODO: See how handle use flags enabled.
			if d.Use != "" {
				for _, d2 := range d.SubDeps {

					//TODO: Resolve to db or create a new one.
					dep := &pkg.DefaultPackage{
						Name:     d2.Name,
						Version:  d2.Version + d2.VersionSuffix,
						Category: d2.Category,
					}
					if d2.Condition == _gentoo.PkgCondNot {
						pack.PackageConflicts = append(pack.PackageConflicts, dep)
					} else {
						pack.PackageRequires = append(pack.PackageRequires, dep)
					}
				}
			} else {

				//TODO: Resolve to db or create a new one.
				dep := &pkg.DefaultPackage{
					Name:     d.Dep.Name,
					Version:  d.Dep.Version + d.Dep.VersionSuffix,
					Category: d.Dep.Category,
				}
				if d.Dep.Condition == _gentoo.PkgCondNot {
					pack.PackageConflicts = append(pack.PackageConflicts, dep)
				} else {
					pack.PackageRequires = append(pack.PackageRequires, dep)
				}
			}
		}

	}
	Debug("Finished processing ebuild", path)

	//TODO: Deps and conflicts
	return []pkg.Package{pack}, nil
}
