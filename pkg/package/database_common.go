// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package pkg

import "github.com/pkg/errors"

func clone(src, dst PackageDatabase) error {
	for _, i := range src.World() {
		_, err := dst.CreatePackage(i)
		if err != nil {
			return errors.Wrap(err, "Failed create package "+i.HumanReadableString())
		}
	}
	return nil
}

func copy(src PackageDatabase) (PackageDatabase, error) {
	dst := NewInMemoryDatabase(false)

	if err := clone(src, dst); err != nil {
		return dst, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	return dst, nil
}
