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
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	pkg "github.com/mudler/luet/pkg/package"
)

// SimpleEbuildParser ignores USE flags and generates just 1-1 package
type SimpleEbuildParser struct {
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
	//TODO: Deps and conflicts
	return []pkg.Package{&pkg.DefaultPackage{Name: packageInfo[0][2], Version: packageInfo[0][7]}}, nil
}
