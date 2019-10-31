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

package solver

import (
	"fmt"

	pkg "github.com/mudler/luet/pkg/package"
)

// PackageAssert represent a package assertion.
// It is composed of a Package and a Value which is indicating the absence or not
// of the associated package state.
type PackageAssert struct {
	Package pkg.Package
	Value   bool
}

// DecodeModel decodes a model from the SAT solver to package assertions (PackageAssert)
func DecodeModel(model map[string]bool) ([]PackageAssert, error) {
	ass := make([]PackageAssert, 0)
	for k, v := range model {
		a, err := pkg.DecodePackage(k)
		if err != nil {
			return nil, err

		}
		ass = append(ass, PackageAssert{Package: a, Value: v})
	}
	return ass, nil
}

func (a *PackageAssert) Explain() {
	fmt.Println(a.ToString())
	a.Package.Explain()
}

func (a *PackageAssert) ToString() string {
	var msg string
	if a.Package.Flagged() {
		msg = "installed"
	} else {
		msg = "not installed"
	}
	return fmt.Sprintf("%s/%s %s %s: %t", a.Package.GetCategory(), a.Package.GetName(), a.Package.GetVersion(), msg, a.Value)
}
