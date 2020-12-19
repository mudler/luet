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

	Create(string, []byte) (string, error)
	Retrieve(ID string) ([]byte, error)
}

type PackageSet interface {
	Clone(PackageDatabase) error
	Copy() (PackageDatabase, error)

	GetRevdeps(p Package) (Packages, error)
	GetPackages() []string //Ids
	CreatePackage(pkg Package) (string, error)
	GetPackage(ID string) (Package, error)
	Clean() error
	FindPackage(Package) (Package, error)
	FindPackages(p Package) (Packages, error)
	UpdatePackage(p Package) error
	GetAllPackages(packages chan Package) error
	RemovePackage(Package) error

	GetPackageFiles(Package) ([]string, error)
	SetPackageFiles(*PackageFile) error
	RemovePackageFiles(Package) error
	FindPackageVersions(p Package) (Packages, error)
	World() Packages

	FindPackageCandidate(p Package) (Package, error)
	FindPackageLabel(labelKey string) (Packages, error)
	FindPackageLabelMatch(pattern string) (Packages, error)
	FindPackageMatch(pattern string) (Packages, error)
}

type PackageFile struct {
	ID                 int `storm:"id,increment"` // primary key with auto increment
	PackageFingerprint string
	Files              []string
}
