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
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "github.com/mudler/luet/pkg/logger"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	pkg "github.com/mudler/luet/pkg/package"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/shell"
	"mvdan.cc/sh/v3/syntax"
)

const (
	uriRegex = "(.*[.]tar[.].*|.*[.]zip|.*[.]run|.*[.]png|.*[.]rpm|.*[.]gz)"
)

// SimpleEbuildParser ignores USE flags and generates just 1-1 package
type SimpleEbuildParser struct {
	World pkg.PackageDatabase
}

type GentooDependency struct {
	Use          string
	UseCondition _gentoo.PackageCond
	SubDeps      []*GentooDependency
	Dep          *_gentoo.GentooPackage
}

type GentooRDEPEND struct {
	Dependencies []*GentooDependency
}

func NewGentooDependency(pkg, use string) (*GentooDependency, error) {
	var err error
	ans := &GentooDependency{
		Use:     use,
		SubDeps: make([]*GentooDependency, 0),
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

		// TODO: Fix this on parsing phase for handle correctly ${PV}
		if strings.HasSuffix(ans.Dep.Name, "-") {
			ans.Dep.Name = ans.Dep.Name[:len(ans.Dep.Name)-1]
		}

	}

	return ans, nil
}

func (d *GentooDependency) String() string {
	if d.Dep != nil {
		return fmt.Sprintf("%s", d.Dep)
	} else {
		return fmt.Sprintf("%s %d %s", d.Use, d.UseCondition, d.SubDeps)
	}
}

func (d *GentooDependency) GetDepsList() []*GentooDependency {
	ans := make([]*GentooDependency, 0)

	if len(d.SubDeps) > 0 {
		for _, d2 := range d.SubDeps {
			list := d2.GetDepsList()
			ans = append(ans, list...)
		}
	}

	if d.Dep != nil {
		ans = append(ans, d)
	}

	return ans
}

func (d *GentooDependency) AddSubDependency(pkg, use string) (*GentooDependency, error) {
	ans, err := NewGentooDependency(pkg, use)
	if err != nil {
		return nil, err
	}

	d.SubDeps = append(d.SubDeps, ans)

	return ans, nil
}

func (r *GentooRDEPEND) GetDependencies() []*GentooDependency {
	ans := make([]*GentooDependency, 0)

	for _, d := range r.Dependencies {
		list := d.GetDepsList()
		ans = append(ans, list...)
	}

	// the same dependency could be available in multiple use flags.
	// It's needed avoid duplicate.
	m := make(map[string]*GentooDependency, 0)

	for _, p := range ans {
		m[p.String()] = p
	}

	ans = make([]*GentooDependency, 0)
	for _, p := range m {
		ans = append(ans, p)
	}

	return ans
}

func ParseRDEPEND(rdepend string) (*GentooRDEPEND, error) {
	var lastdep []*GentooDependency = make([]*GentooDependency, 0)
	var pendingDep = false
	var orDep = false
	var dep *GentooDependency
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

			if strings.HasPrefix(rr, "|| (") {
				orDep = true
				continue
			}

			if orDep {
				rr = strings.TrimSpace(rr)
				if rr == ")" {
					orDep = false
				}
				continue
			}

			if strings.Index(rr, "?") > 0 {
				// use flag present

				if pendingDep {
					dep, err = lastdep[len(lastdep)-1].AddSubDependency("", rr[:strings.Index(rr, "?")])
					if err != nil {
						Debug("Ignoring subdependency ", rr[:strings.Index(rr, "?")])
					}
				} else {
					dep, err = NewGentooDependency("", rr[:strings.Index(rr, "?")])
					if err != nil {
						Debug("Ignoring dep", rr)
					} else {
						ans.Dependencies = append(ans.Dependencies, dep)
					}
				}

				if strings.Index(rr, ")") < 0 {
					pendingDep = true
					lastdep = append(lastdep, dep)
				}

				if strings.Index(rr, "|| (") >= 0 {
					// Ignore dep in or
					continue
				}

				fields := strings.Split(rr[strings.Index(rr, "?")+1:], " ")
				for _, f := range fields {
					f = strings.TrimSpace(f)
					if f == ")" || f == "(" || f == "" {
						continue
					}

					_, err = dep.AddSubDependency(f, "")
					if err != nil {
						Debug("Ignoring subdependency ", f)
					}
				}

			} else if pendingDep {
				fields := strings.Split(rr, " ")
				for _, f := range fields {
					f = strings.TrimSpace(f)
					if f == ")" || f == "(" || f == "" {
						continue
					}
					_, err = lastdep[len(lastdep)-1].AddSubDependency(f, "")
					if err != nil {
						return nil, err
					}
				}

				if strings.Index(rr, ")") >= 0 {
					lastdep = lastdep[:len(lastdep)-1]
					if len(lastdep) == 0 {
						pendingDep = false
					}
				}

			} else {
				rr = strings.TrimSpace(rr)
				// Check if there multiple deps in single row

				fields := strings.Split(rr, " ")
				if len(fields) > 1 {
					for _, rrr := range fields {
						rrr = strings.TrimSpace(rrr)
						if rrr == "" {
							continue
						}
						dep, err := NewGentooDependency(rrr, "")
						if err != nil {
							Debug("Ignoring dep", rr)
						} else {
							ans.Dependencies = append(ans.Dependencies, dep)
						}
					}
				} else {
					dep, err := NewGentooDependency(rr, "")
					if err != nil {
						Debug("Ignoring dep", rr)
					} else {
						ans.Dependencies = append(ans.Dependencies, dep)
					}
				}
			}

		}

	}

	return ans, nil
}

func SourceFile(ctx context.Context, path string, pkg *_gentoo.GentooPackage) (map[string]expand.Variable, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open: %v", err)
	}
	scontent := string(content)

	// Add default Genoo Variables
	ebuild := fmt.Sprintf("P=%s\n", pkg.GetP()) +
		fmt.Sprintf("PN=%s\n", pkg.GetPN()) +
		fmt.Sprintf("PV=%s\n", pkg.GetPV()) +
		fmt.Sprintf("PVR=%s\n", pkg.GetPVR())

	// Disable inherit
	scontent = strings.ReplaceAll(scontent, "inherit", "#inherit")
	// Disable function from eclass (TODO: check how fix better this)
	scontent = strings.ReplaceAll(scontent, "need_apache", "#need_apache")
	scontent = strings.ReplaceAll(scontent, "want_apache", "#want_apache")

	regexFuncs := regexp.MustCompile(
		"[a-zA-Z]+.*[_][a-z]+[(][)][\\s]{",
	)
	matches := regexFuncs.FindAllIndex([]byte(scontent), -1)
	// Drop section after functions (src_*, *() {)
	if len(matches) > 0 {
		ebuild = ebuild + scontent[:matches[0][0]]
	} else {
		ebuild = ebuild + scontent
	}

	// [[ ${PV} == "9999" ]] is not supported. Workaround but we need a better solution.
	regexDoubleBrakets := regexp.MustCompile(
		//"[[][[].*",
		"^[[][[].*",
		//"^.*\[\[.*\]\]",
	)
	matchDB := regexDoubleBrakets.FindAllIndex([]byte(ebuild), -1)
	if len(matchDB) > 0 {
		ebuild = ebuild[:matchDB[0][0]] + "#" + ebuild[matchDB[0][0]:]
	}

	//fmt.Println("EBUILD ", ebuild)

	file, err := syntax.NewParser().Parse(strings.NewReader(ebuild), path)
	if err != nil {
		return nil, fmt.Errorf("could not parse: %v", err)
	}
	return shell.SourceNode(ctx, file)
}

// ScanEbuild returns a list of packages (always one with SimpleEbuildParser) decoded from an ebuild.
func (ep *SimpleEbuildParser) ScanEbuild(path string) (pkg.Packages, error) {
	Debug("Starting parsing of ebuild", path)

	pkgstr := filepath.Base(path)
	paths := strings.Split(filepath.Dir(path), "/")
	pkgstr = paths[len(paths)-2] + "/" + strings.Replace(pkgstr, ".ebuild", "", -1)

	gp, err := _gentoo.ParsePackageStr(pkgstr)
	if err != nil {
		return pkg.Packages{}, errors.New("Error on parsing package string")
	}

	pack := &pkg.DefaultPackage{
		Name:     gp.Name,
		Version:  fmt.Sprintf("%s%s", gp.Version, gp.VersionSuffix),
		Category: gp.Category,
		Uri:      make([]string, 0),
	}

	Debug("Prepare package ", pack.Category+"/"+pack.Name+"-"+pack.Version)

	// Adding a timeout of 60secs, as with some bash files it can hang indefinetly
	timeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	vars, err := SourceFile(timeout, path, gp)
	if err != nil {
		Error("Error on source file ", pack.Name, ": ", err)
		return pkg.Packages{}, err
	}

	// TODO: Handle this a bit better
	iuse, ok := vars["IUSE"]
	if ok {
		uses := strings.Split(strings.TrimSpace(iuse.String()), " ")
		for _, u := range uses {
			pack.AddUse(u)
		}
	}

	// Retrieve package description
	descr, ok := vars["DESCRIPTION"]
	if ok {
		pack.SetDescription(descr.String())
	}
	// Retrieve package license
	license, ok := vars["LICENSE"]
	if ok {
		pack.SetLicense(license.String())
	}
	uri, ok := vars["SRC_URI"]
	if ok {
		// TODO: handle mirror:
		uris := strings.Split(uri.String(), "\n")
		for _, u := range uris {
			u = strings.TrimSpace(u)

			if u == "" {
				continue
			}
			if match, _ := regexp.Match(uriRegex, []byte(u)); match {
				if strings.Index(u, "(") >= 0 {
					regexUri := regexp.MustCompile("(http|ftp|mirror).*[ ]")
					matches := regexUri.FindAllIndex([]byte(u), -1)
					if len(matches) > 0 {
						u = u[matches[0][0]:matches[0][1]]
					} else {
						continue
					}
				}
				pack.AddURI(u)
				Debug("Add uri ", u)
			} else {
				Debug("Skip uri ", u)
			}
		}
	}

	rdepend, ok := vars["RDEPEND"]
	if ok {
		gRDEPEND, err := ParseRDEPEND(rdepend.String())
		if err != nil {
			Warning("Error on parsing RDEPEND for package ", pack.Category+"/"+pack.Name, err)
			return pkg.Packages{pack}, nil
			//	return pkg.Packages{}, err
		}

		pack.PackageConflicts = []*pkg.DefaultPackage{}
		pack.PackageRequires = []*pkg.DefaultPackage{}

		// TODO: See how handle use flags enabled.
		// and if it's correct get list of deps directly.
		for _, d := range gRDEPEND.GetDependencies() {

			//TODO: Resolve to db or create a new one.
			dep := &pkg.DefaultPackage{
				Name:     d.Dep.Name,
				Version:  d.Dep.Version + d.Dep.VersionSuffix,
				Category: d.Dep.Category,
			}
			Debug(fmt.Sprintf("For package %s found dep: %s/%s %s",
				gp, dep.Category, dep.Name, dep.Version))
			if d.Dep.Condition == _gentoo.PkgCondNot {
				pack.PackageConflicts = append(pack.PackageConflicts, dep)
			} else {
				pack.PackageRequires = append(pack.PackageRequires, dep)
			}

		}

	}
	Debug("Finished processing ebuild", path, "deps ", len(pack.PackageRequires))

	//TODO: Deps and conflicts
	return pkg.Packages{pack}, nil
}
