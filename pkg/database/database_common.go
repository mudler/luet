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

package database

import (
	stderrors "errors"
	"regexp"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// Sentinel errors for lookup misses.
//
// These are returned on paths that are hit constantly during resolution -
// getProvide() runs at the top of every FindPackage/FindPackages/
// FindPackageVersions, and the overwhelmingly common outcome is "not found",
// which is a control-flow signal rather than an exceptional condition.
//
// They are deliberately built with the standard library instead of
// github.com/pkg/errors: the latter's New() calls runtime.Callers to capture a
// 32-frame stack trace on every invocation, which profiled at ~46% of CPU time
// during an upgrade. Declaring them once at package level makes a miss free.
//
// Compare with errors.Is - do not match on the message text.
var (
	// ErrKeyNotFound is returned when a raw key lookup misses.
	ErrKeyNotFound = stderrors.New("no key found")
	// ErrNoVersionsFound is returned when a package name has no known versions.
	ErrNoVersionsFound = stderrors.New("no versions found for package")
	// ErrNoProvider is returned when no package provides the requested one.
	ErrNoProvider = stderrors.New("no package provides this")
)

func clone(src, dst types.PackageDatabase) error {
	for _, i := range src.World() {
		_, err := dst.CreatePackage(i)
		if err != nil {
			return errors.Wrap(err, "Failed create package "+i.HumanReadableString())
		}
	}
	return nil
}

func copy(src types.PackageDatabase) (types.PackageDatabase, error) {
	dst := NewInMemoryDatabase(false)

	if err := clone(src, dst); err != nil {
		return dst, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	return dst, nil
}

func findPackageByFile(db types.PackageDatabase, pattern string) (types.Packages, error) {

	var ans []*types.Package

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid regex "+pattern+"!")
	}

PACKAGE:
	for _, pack := range db.World() {
		files, err := db.GetPackageFiles(pack)
		if err == nil {
			for _, f := range files {
				if re.MatchString(f) {
					ans = append(ans, pack)
					continue PACKAGE
				}
			}
		}
	}

	return types.Packages(ans), nil

}
