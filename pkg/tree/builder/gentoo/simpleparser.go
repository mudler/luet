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
	"regexp"
	"strings"

	pkg "github.com/mudler/luet/pkg/package"
	"mvdan.cc/sh/expand"
	"mvdan.cc/sh/shell"
	"mvdan.cc/sh/syntax"
)

// SimpleEbuildParser ignores USE flags and generates just 1-1 package
type SimpleEbuildParser struct {
	World pkg.PackageDatabase
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
func (ep *SimpleEbuildParser) ScanEbuild(path string) ([]pkg.Package, error) {

	file := filepath.Base(path)
	file = strings.Replace(file, ".ebuild", "", -1)

	decodepackage, err := regexp.Compile(`^([<>]?=?)((([^\/]+)\/)?(?U)(\S+))(-(\d+(\.\d+)*[a-z]?(_(alpha|beta|pre|rc|p)\d*)*(-r\d+)?))?$`)
	if err != nil {
		return []pkg.Package{}, errors.New("Invalid regex")

	}
	packageInfo := decodepackage.FindAllStringSubmatch(file, -1)
	if len(packageInfo) != 1 || len(packageInfo[0]) != 12 {
		return []pkg.Package{}, errors.New("Failed decoding ebuild: " + path)
	}

	vars, err := SourceFile(context.TODO(), path)
	if err != nil {
		//	return []pkg.Package{}, err
	}
	//fmt.Println("Scanning", path)
	//fmt.Println(vars)
	pack := &pkg.DefaultPackage{Name: packageInfo[0][2], Version: packageInfo[0][7]}
	rdepend, ok := vars["RDEPEND"]
	if ok {
		rdepends := strings.Split(rdepend.String(), "\n")

		pack.PackageRequires = []*pkg.DefaultPackage{}
		for _, rr := range rdepends {

			//TODO: Resolve to db or create a new one.
			pack.PackageRequires = append(pack.PackageRequires, &pkg.DefaultPackage{Name: rr})
		}

	}

	//TODO: Deps and conflicts
	return []pkg.Package{pack}, nil
}
