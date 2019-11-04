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

// Recipe is a builder imeplementation.

// It reads a Tree and spit it in human readable form (YAML), called recipe,
// It also loads a tree (recipe) from a YAML (to a db, e.g. BoltDB), allowing to query it
// with the solver, using the package object.
package tree

import (
	"errors"
	"sync"

	. "github.com/mudler/luet/pkg/logger"

	pkg "github.com/mudler/luet/pkg/package"
)

func NewDefaultTree() pkg.Tree { return &DefaultTree{} }

type DefaultTree struct {
	Packages   pkg.PackageSet
	CacheWorld []pkg.Package
}

func (gt *DefaultTree) GetPackageSet() pkg.PackageSet {
	return gt.Packages
}

func (gt *DefaultTree) Prelude() string {
	return ""
}

func (gt *DefaultTree) SetPackageSet(s pkg.PackageSet) {
	gt.Packages = s
}

func (gt *DefaultTree) World() ([]pkg.Package, error) {
	if len(gt.CacheWorld) > 0 {
		return gt.CacheWorld, nil
	}
	packages := []pkg.Package{}
	for _, pid := range gt.GetPackageSet().GetPackages() {

		p, err := gt.GetPackageSet().GetPackage(pid)
		if err != nil {
			return packages, err
		}
		packages = append(packages, p)
	}
	gt.CacheWorld = packages
	return packages, nil
}

// FIXME: Dup in Packageset
func (gt *DefaultTree) FindPackage(pack pkg.Package) (pkg.Package, error) {
	packages, err := gt.World()
	if err != nil {
		return nil, err
	}
	for _, pid := range packages {
		if pack.GetFingerPrint() == pid.GetFingerPrint() {
			return pid, nil
		}
	}
	return nil, errors.New("No package found")
}

func (gb *DefaultTree) depsWorker(i int, wg *sync.WaitGroup, c <-chan pkg.Package) error {
	defer wg.Done()

	for p := range c {
		SpinnerText(" "+p.GetName(), "Deps ")
		for _, r := range p.GetRequires() {

			foundPackage, err := gb.GetPackageSet().FindPackage(r)
			if err == nil {

				found, ok := foundPackage.(*pkg.DefaultPackage)
				if !ok {
					panic("Simpleparser should deal only with DefaultPackages")
				}
				r = found
			} else {
				Warning("Unmatched require for", r.GetName())
			}
		}

		Debug("Walking conflicts for", p.GetName())
		for _, r := range p.GetConflicts() {
			Debug("conflict", r.GetName())

			foundPackage, err := gb.GetPackageSet().FindPackage(r)
			if err == nil {

				found, ok := foundPackage.(*pkg.DefaultPackage)
				if !ok {
					panic("Simpleparser should deal only with DefaultPackages")
				}
				r = found
			} else {
				Warning("Unmatched conflict for", r.GetName())

			}
		}
		Debug("Finished processing", p.GetName())

		if err := gb.GetPackageSet().UpdatePackage(p); err != nil {
			return err
		}
		Debug("Update done", p.GetName())

	}

	return nil
}

// Search for deps/conflicts in db and replaces it with packages in the db
func (t *DefaultTree) ResolveDeps(concurrency int) error {
	all := make(chan pkg.Package)

	var wg = new(sync.WaitGroup)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go t.depsWorker(i, wg, all)
	}

	err := t.GetPackageSet().GetAllPackages(all)
	close(all)
	wg.Wait()
	return err
}
