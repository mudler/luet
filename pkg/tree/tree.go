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
	"sync"

	. "github.com/mudler/luet/pkg/logger"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"

	pkg "github.com/mudler/luet/pkg/package"
)

func NewDefaultTree() pkg.Tree { return &DefaultTree{} }

type DefaultTree struct {
	sync.Mutex
	Packages   pkg.PackageDatabase
	CacheWorld []pkg.Package
}

func (gt *DefaultTree) GetPackageSet() pkg.PackageDatabase {
	return gt.Packages
}

func (gt *DefaultTree) Prelude() string {
	return ""
}

func (gt *DefaultTree) SetPackageSet(s pkg.PackageDatabase) {
	gt.Packages = s
}

func (gt *DefaultTree) World() ([]pkg.Package, error) {
	gt.Lock()
	defer gt.Unlock()
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

func (gt *DefaultTree) UpdateWorldPackage(p pkg.Package) {
	gt.Lock()
	defer gt.Unlock()
	//var CacheWorld []pkg.Package
	for _, pid := range gt.CacheWorld {
		if p.Matches(pid) {
			pid.Requires(p.GetRequires())
			pid.Conflicts(p.GetConflicts())
		}
	}

}

// FIXME: Dup in Packageset
func (gt *DefaultTree) FindPackage(pack pkg.Package) (pkg.Package, error) {
	packages, err := gt.World()
	if err != nil {
		return nil, err
	}
	for _, pid := range packages {
		if pack.Matches(pid) {
			return pid, nil
		}
	}
	return nil, errors.New("No package found")
}

func (gb *DefaultTree) updatePackage(p pkg.Package) error {
	Debug("🔧 Calculating deps for", p.GetName())
	for i, r := range p.GetRequires() {
		foundPackage, err := gb.FindPackage(r)
		if err == nil {
			found, ok := foundPackage.(*pkg.DefaultPackage)
			if !ok {
				return errors.New("Simpleparser should deal only with DefaultPackages")
			}

			p.GetRequires()[i] = found
		} else {
			Warning("Unmatched require for", r.GetFingerPrint())
			w, err := gb.World()
			if err != nil {
				return errors.Wrap(err, "Error while computing world")
			}
			packages, err := r.Expand(&w)
			if err != nil {
				return errors.Wrap(err, "Error while expanding to world")
			}
			if len(packages) == 0 {
				Warning("Could not expand")
				continue
			}

			s := solver.NewSolver([]pkg.Package{}, []pkg.Package{}, gb.GetPackageSet())
			best := s.Best(packages)
			found, ok := best.(*pkg.DefaultPackage)
			if !ok {
				return errors.New("Simpleparser should deal only with DefaultPackages")
			}
			Info(":ok_hand: Automatically selected", found.GetName(), found.GetVersion())
			p.GetRequires()[i] = found
		}
	}

	Debug("🔍 Walking conflicts for", p.GetName())
	for i, r := range p.GetConflicts() {
		foundPackage, err := gb.FindPackage(r)
		if err == nil {
			found, ok := foundPackage.(*pkg.DefaultPackage)
			if !ok {
				return errors.New("Simpleparser should deal only with DefaultPackages")
			}

			p.GetConflicts()[i] = found
		} else {
			Warning("Unmatched conflict for", r.GetFingerPrint())
			w, err := gb.World()
			if err != nil {
				return errors.Wrap(err, "Error while computing world")
			}
			packages, err := r.Expand(&w)
			if err != nil {
				return errors.Wrap(err, "Error while expanding to world")
			}
			if len(packages) == 0 {
				Warning("Could not expand")
				continue
			}

			confs := []*pkg.DefaultPackage{}
			for _, c := range packages {
				found, ok := c.(*pkg.DefaultPackage)
				if !ok {
					return errors.New("Simpleparser should deal only with DefaultPackages")
				}
				confs = append(confs, found)
			}
			p.Conflicts(append(p.GetConflicts(), confs...))

		}
	}
	//	Debug("💫 Finished processing", p.GetName())

	if err := gb.GetPackageSet().UpdatePackage(p); err != nil {
		return err
	}

	gb.UpdateWorldPackage(p)
	//Debug("Update done", p.GetName())
	// Debug("Triggering propagation", p.GetName())

	// Debug(" "+p.GetName(), "Deps ")
	// for _, r := range p.GetRequires() {
	// 	if err := gb.updatePackage(r); err != nil {
	// 		return err
	// 	}
	// }

	// Debug("Walking conflicts for", p.GetName())
	// for _, r := range p.GetConflicts() {
	// 	if err := gb.updatePackage(r); err != nil {
	// 		return err
	// 	}
	// }

	return nil

}

func (gb *DefaultTree) depsWorker(i int, wg *sync.WaitGroup, c <-chan pkg.Package) error {
	defer wg.Done()

	for p := range c {
		gb.updatePackage(p)
	}

	return nil
}

// Search for deps/conflicts in db and replaces it with packages in the db
func (t *DefaultTree) ResolveDeps(concurrency int) error {
	Spinner(32)
	defer SpinnerStop()
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
