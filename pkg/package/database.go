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

package pkg

// Database is a merely simple in-memory db.
// FIXME: Use a proper structure or delegate to third-party
type PackageDatabase interface {
	PackageSet

	Get(s string) (string, error)
	Set(k, v string) error

	Create([]byte) (string, error)
	Retrieve(ID string) ([]byte, error)
}
