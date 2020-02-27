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

	//. "github.com/mudler/luet/pkg/logger"
	"github.com/pkg/errors"

	"github.com/crillab/gophersat/bf"
	pkg "github.com/mudler/luet/pkg/package"
)

// PackageSolver is an interface to a generic package solving algorithm
type PackageSolver interface {
	SetDefinitionDatabase(pkg.PackageDatabase)
	Install(p []pkg.Package) (PackagesAssertions, error)
	Uninstall(candidate pkg.Package, checkconflicts bool) ([]pkg.Package, error)
	ConflictsWithInstalled(p pkg.Package) (bool, error)
	ConflictsWith(p pkg.Package, ls []pkg.Package) (bool, error)
	World() []pkg.Package
	Upgrade(checkconflicts bool) ([]pkg.Package, PackagesAssertions, error)

	SetResolver(PackageResolver)

	Solve() (PackagesAssertions, error)
}

// Solver is the default solver for luet
type Solver struct {
	DefinitionDatabase pkg.PackageDatabase
	SolverDatabase     pkg.PackageDatabase
	Wanted             []pkg.Package
	InstalledDatabase  pkg.PackageDatabase

	Resolver PackageResolver
}

// NewSolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
func NewSolver(installed pkg.PackageDatabase, definitiondb pkg.PackageDatabase, solverdb pkg.PackageDatabase) PackageSolver {
	return NewResolver(installed, definitiondb, solverdb, &DummyPackageResolver{})
}

// NewReSolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
func NewResolver(installed pkg.PackageDatabase, definitiondb pkg.PackageDatabase, solverdb pkg.PackageDatabase, re PackageResolver) PackageSolver {
	return &Solver{InstalledDatabase: installed, DefinitionDatabase: definitiondb, SolverDatabase: solverdb, Resolver: re}
}

// SetDefinitionDatabase is a setter for the definition Database

func (s *Solver) SetDefinitionDatabase(db pkg.PackageDatabase) {
	s.DefinitionDatabase = db
}

// SetResolver is a setter for the unsat resolver backend
func (s *Solver) SetResolver(r PackageResolver) {
	s.Resolver = r
}

func (s *Solver) World() []pkg.Package {
	return s.DefinitionDatabase.World()
}

func (s *Solver) Installed() []pkg.Package {

	return s.InstalledDatabase.World()
}

func (s *Solver) noRulesWorld() bool {
	for _, p := range s.World() {
		if len(p.GetConflicts()) != 0 || len(p.GetRequires()) != 0 {
			return false
		}
	}

	return true
}

func (s *Solver) BuildInstalled() (bf.Formula, error) {
	var formulas []bf.Formula
	for _, p := range s.Installed() {
		solvable, err := p.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
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

	for _, p := range s.World() {
		solvable, err := p.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}
	return bf.And(formulas...), nil
}

func (s *Solver) getList(db pkg.PackageDatabase, lsp []pkg.Package) ([]pkg.Package, error) {
	var ls []pkg.Package

	for _, pp := range lsp {
		cp, err := db.FindPackage(pp)
		if err != nil {
			packages, err := pp.Expand(db)
			// Expand, and relax search - if not found pick the same one
			if err != nil || len(packages) == 0 {
				cp = pp
			} else {
				cp = pkg.Best(packages)
			}
		}
		ls = append(ls, cp)
	}
	return ls, nil
}

func (s *Solver) ConflictsWith(pack pkg.Package, lsp []pkg.Package) (bool, error) {
	p, err := s.DefinitionDatabase.FindPackage(pack)
	if err != nil {
		p = pack //Relax search, otherwise we cannot compute solutions for packages not in definitions

		//	return false, errors.Wrap(err, "Package not found in definition db")
	}

	ls, err := s.getList(s.DefinitionDatabase, lsp)
	if err != nil {
		return false, errors.Wrap(err, "Package not found in definition db")
	}

	var formulas []bf.Formula

	if s.noRulesWorld() {
		return false, nil
	}

	encodedP, err := p.Encode(s.SolverDatabase)
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

		encodedI, err := i.Encode(s.SolverDatabase)
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
	return s.ConflictsWith(p, s.Installed())
}

func (s *Solver) Upgrade(checkconflicts bool) ([]pkg.Package, PackagesAssertions, error) {

	// First get candidates that needs to be upgraded..

	toUninstall := []pkg.Package{}
	toInstall := []pkg.Package{}

	availableCache := map[string][]pkg.Package{}
	for _, p := range s.DefinitionDatabase.World() {
		// Each one, should be expanded
		availableCache[p.GetName()+p.GetCategory()] = append(availableCache[p.GetName()+p.GetCategory()], p)
	}

	installedcopy := pkg.NewInMemoryDatabase(false)

	for _, p := range s.InstalledDatabase.World() {
		installedcopy.CreatePackage(p)
		packages, ok := availableCache[p.GetName()+p.GetCategory()]
		if ok && len(packages) != 0 {
			best := pkg.Best(packages)
			if best.GetVersion() != p.GetVersion() {
				toUninstall = append(toUninstall, p)
				toInstall = append(toInstall, best)
			}
		}
	}
	s2 := NewSolver(installedcopy, s.DefinitionDatabase, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)
	// Then try to uninstall the versions in the system, and store that tree
	for _, p := range toUninstall {
		r, err := s.Uninstall(p, checkconflicts)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't uninstall selected candidate "+p.GetFingerPrint())
		}
		for _, z := range r {
			err = installedcopy.RemovePackage(z)
			if err != nil {
				return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't remove copy of package targetted for removal")
			}
		}

	}
	r, e := s2.Install(toInstall)
	return toUninstall, r, e
	// To that tree, ask to install the versions that should be upgraded, and try to solve
	// Return the solution

}

// Uninstall takes a candidate package and return a list of packages that would be removed
// in order to purge the candidate. Returns error if unsat.
func (s *Solver) Uninstall(c pkg.Package, checkconflicts bool) ([]pkg.Package, error) {
	var res []pkg.Package
	candidate, err := s.InstalledDatabase.FindPackage(c)
	if err != nil {

		//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
		packages, err := c.Expand(s.InstalledDatabase)
		//	Info("Expanded", packages, err)
		if err != nil || len(packages) == 0 {
			candidate = c
		} else {
			candidate = pkg.Best(packages)
		}
		//Relax search, otherwise we cannot compute solutions for packages not in definitions
		//	return nil, errors.Wrap(err, "Package not found between installed")
	}
	// Build a fake "Installed" - Candidate and its requires tree
	var InstalledMinusCandidate []pkg.Package

	// TODO: Can be optimized
	for _, i := range s.Installed() {
		if !i.Matches(candidate) {
			contains, err := candidate.RequiresContains(s.SolverDatabase, i)
			if err != nil {
				return nil, errors.Wrap(err, "Failed getting installed list")
			}
			if !contains {
				InstalledMinusCandidate = append(InstalledMinusCandidate, i)
			}
		}
	}

	s2 := NewSolver(pkg.NewInMemoryDatabase(false), s.DefinitionDatabase, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)
	// Get the requirements to install the candidate
	asserts, err := s2.Install([]pkg.Package{candidate})
	if err != nil {
		return nil, err
	}
	for _, a := range asserts {
		if a.Value {
			if !checkconflicts {
				res = append(res, a.Package.IsFlagged(false))
				continue
			}

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
		encodedW, err := wanted.Encode(s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		W := bf.Var(encodedW)
		installedWorld := s.Installed()
		//TODO:Optimize
		if len(installedWorld) == 0 {
			formulas = append(formulas, W) //bf.And(bf.True, W))
			continue
		}

		for _, installed := range installedWorld {
			encodedI, err := installed.Encode(s.SolverDatabase)
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
	var model map[string]bool
	var err error

	f, err := s.BuildFormula()

	if err != nil {
		return nil, err
	}

	model, _, err = s.solve(f)
	if err != nil && s.Resolver != nil {
		return s.Resolver.Solve(f, s)
	}

	if err != nil {
		return nil, err
	}

	return DecodeModel(model, s.SolverDatabase)
}

// Install given a list of packages, returns package assertions to indicate the packages that must be installed in the system in order
// to statisfy all the constraints
func (s *Solver) Install(c []pkg.Package) (PackagesAssertions, error) {

	coll, err := s.getList(s.DefinitionDatabase, c)
	if err != nil {
		return nil, errors.Wrap(err, "Packages not found in definition db")
	}

	s.Wanted = coll

	if s.noRulesWorld() {
		var ass PackagesAssertions
		for _, p := range s.Installed() {
			ass = append(ass, PackageAssert{Package: p.(*pkg.DefaultPackage), Value: true})

		}
		for _, p := range s.Wanted {
			ass = append(ass, PackageAssert{Package: p.(*pkg.DefaultPackage), Value: true})
		}
		return ass, nil
	}

	return s.Solve()
}
