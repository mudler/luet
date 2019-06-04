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
	pkg "gitlab.com/mudler/luet/pkg/package"
)

type PackageAssert struct {
	Package pkg.Package
	Value   bool
}

func DecodeModel(model map[string]bool) ([]PackageAssert, error) {
	ass := make([]PackageAssert, 0)
	for k, v := range model {
		if a, err := pkg.DecodePackage(k); err == nil {

			// fmt.Println("Flagged", v, a.Flagged())
			// if v {
			// 	fmt.Println("To flag", a)
			// }
			// if a.Flagged() && !v {
			// 	a.IsFlagged(false)
			// } else if !a.Flagged() && v {
			// 	fmt.Println("To flag ", a)
			// 	a.IsFlagged(true)
			// }

			//if a.State == common.STATE_CURRENT {
			ass = append(ass, PackageAssert{Package: a, Value: v})
			//} // Else, there was a state transition between Initial state and current run
		} else {
			return nil, err
		}
	}
	return ass, nil
}
