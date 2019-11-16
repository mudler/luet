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
	"errors"
	"sort"

	"github.com/crillab/gophersat/bf"
	"github.com/hashicorp/go-version"
	pkg "github.com/mudler/luet/pkg/package"
)

// PackageSolver is an interface to a generic package solving algorithm
type PackageSolver interface {
	SetWorld(p []pkg.Package)
	Install(p []pkg.Package) (PackagesAssertions, error)
	Uninstall(candidate pkg.Package) ([]pkg.Package, error)
	ConflictsWithInstalled(p pkg.Package) (bool, error)
	ConflictsWith(p pkg.Package, ls []pkg.Package) (bool, error)
	Best([]pkg.Package) pkg.Package
}

// Solver is the default solver for luet
type Solver struct {
	Database  pkg.PackageDatabase
	Wanted    []pkg.Package
	Installed []pkg.Package
	World     []pkg.Package
}

// NewSolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
func NewSolver(init []pkg.Package, w []pkg.Package, db pkg.PackageDatabase) PackageSolver {
	for _, v := range init {
		pkg.NormalizeFlagged(v)
	}
	for _, v := range w {
		pkg.NormalizeFlagged(v)
	}
	return &Solver{Installed: init, World: w, Database: db}
}

// TODO: []pkg.Package should have its own type with this kind of methods in (+Unique, sort, etc.)
func (s *Solver) Best(set []pkg.Package) pkg.Package {
	var versionsMap map[string]pkg.Package = make(map[string]pkg.Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		versionsRaw = append(versionsRaw, p.GetVersion())
		versionsMap[p.GetVersion()] = p
	}

	versions := make([]*version.Version, len(versionsRaw))
	for i, raw := range versionsRaw {
		v, _ := version.NewVersion(raw)
		versions[i] = v
	}

	// After this, the versions are properly sorted
	sort.Sort(version.Collection(versions))

	return versionsMap[versions[len(versions)-1].Original()]
}

// SetWorld is a setter for the list of all known packages to the solver

func (s *Solver) SetWorld(p []pkg.Package) {
	s.World = p
}

func (s *Solver) noRulesWorld() bool {
	for _, p := range s.World {
		if len(p.GetConflicts()) != 0 || len(p.GetRequires()) != 0 {
			return false
		}
	}

	return true
}

func (s *Solver) BuildInstalled() (bf.Formula, error) {
	var formulas []bf.Formula
	for _, p := range s.Installed {
		solvable, err := p.BuildFormula(s.Database)
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable...)

	}
	return bf.And(formulas...), nil

}

// BuildWorld builds the formula which olds the requirements from the package definitions
// which are available (global state)
func (s *Solver) BuildWorld(includeInstalled bool) (bf.Formula, error) {
	var formulas []bf.Formula
	// NOTE: This block should be enabled in case of very old systems with outdated world sets
	if includeInstalled {
		solvable, err := s.BuildInstalled()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable)
	}

	for _, p := range s.World {
		solvable, err := p.BuildFormula(s.Database)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}
	return bf.And(formulas...), nil
}

func (s *Solver) ConflictsWith(p pkg.Package, ls []pkg.Package) (bool, error) {
	pkg.NormalizeFlagged(p)
	var formulas []bf.Formula

	if s.noRulesWorld() {
		return false, nil
	}

	encodedP, err := p.IsFlagged(true).Encode(s.Database)
	if err != nil {
		return false, err
	}
	P := bf.Var(encodedP)

	r, err := s.BuildWorld(false)
	if err != nil {
		return false, err
	}
	formulas = append(formulas, bf.And(bf.Not(P), r))

	for _, i := range ls {
		if i.Matches(p) {
			continue
		}
		// XXX: Skip check on any of its requires ?  ( Drop to avoid removing system packages when selecting an uninstall)
		// if i.RequiresContains(p) {
		// 	fmt.Println("Requires found")
		// 	continue
		// }

		encodedI, err := i.Encode(s.Database)
		if err != nil {
			return false, err
		}
		I := bf.Var(encodedI)
		formulas = append(formulas, bf.And(I, r))
	}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return true, nil
	}

	return false, nil

}

func (s *Solver) ConflictsWithInstalled(p pkg.Package) (bool, error) {
	return s.ConflictsWith(p, s.Installed)
}

// Uninstall takes a candidate package and return a list of packages that would be removed
// in order to purge the candidate. Returns error if unsat.
func (s *Solver) Uninstall(candidate pkg.Package) ([]pkg.Package, error) {
	var res []pkg.Package

	// Build a fake "Installed" - Candidate and its requires tree
	var InstalledMinusCandidate []pkg.Package
	for _, i := range s.Installed {
		if !i.Matches(candidate) && !candidate.RequiresContains(i) {
			InstalledMinusCandidate = append(InstalledMinusCandidate, i)
		}
	}

	// Get the requirements to install the candidate
	saved := s.Installed
	s.Installed = []pkg.Package{}
	asserts, err := s.Install([]pkg.Package{candidate})
	if err != nil {
		return nil, err
	}
	s.Installed = saved

	for _, a := range asserts {
		if a.Value && a.Package.Flagged() {

			c, err := s.ConflictsWithInstalled(a.Package)
			if err != nil {
				return nil, err
			}

			// If doesn't conflict with installed we just consider it for removal and look for the next one
			if !c {
				res = append(res, a.Package.IsFlagged(false))
				continue
			}

			// If does conflicts, give it another chance by checking conflicts if in case we didn't installed our candidate and all the required packages in the system
			c, err = s.ConflictsWith(a.Package, InstalledMinusCandidate)
			if err != nil {
				return nil, err
			}
			if !c {
				res = append(res, a.Package.IsFlagged(false))
			}

		}

	}

	return res, nil
}

// BuildFormula builds the main solving formula that is evaluated by the sat solver.
func (s *Solver) BuildFormula() (bf.Formula, error) {
	var formulas []bf.Formula
	r, err := s.BuildWorld(false)
	if err != nil {
		return nil, err
	}
	for _, wanted := range s.Wanted {
		encodedW, err := wanted.Encode(s.Database)
		if err != nil {
			return nil, err
		}
		W := bf.Var(encodedW)

		if len(s.Installed) == 0 {
			formulas = append(formulas, W) //bf.And(bf.True, W))
			continue
		}

		for _, installed := range s.Installed {
			encodedI, err := installed.Encode(s.Database)
			if err != nil {
				return nil, err
			}
			I := bf.Var(encodedI)
			formulas = append(formulas, bf.And(W, I))
		}

	}
	formulas = append(formulas, r)

	return bf.And(formulas...), nil
}

func (s *Solver) solve(f bf.Formula) (map[string]bool, bf.Formula, error) {
	model := bf.Solve(f)
	if model == nil {
		return model, f, errors.New("Unsolvable")
	}

	return model, f, nil
}

// Solve builds the formula given the current state and returns package assertions
func (s *Solver) Solve() (PackagesAssertions, error) {
	f, err := s.BuildFormula()

	if err != nil {
		return nil, err
	}

	model, _, err := s.solve(f)
	if err != nil {
		return nil, err
	}

	return DecodeModel(model, s.Database)
}

// Install given a list of packages, returns package assertions to indicate the packages that must be installed in the system in order
// to statisfy all the constraints
func (s *Solver) Install(coll []pkg.Package) (PackagesAssertions, error) {
	for _, v := range coll {
		v.IsFlagged(false)
	}
	s.Wanted = coll

	if s.noRulesWorld() {
		var ass PackagesAssertions
		for _, p := range s.Installed {
			ass = append(ass, PackageAssert{Package: p.IsFlagged(true), Value: true})

		}
		for _, p := range s.Wanted {
			ass = append(ass, PackageAssert{Package: p.IsFlagged(true), Value: true})
		}
		return ass, nil
	}

	return s.Solve()
}
