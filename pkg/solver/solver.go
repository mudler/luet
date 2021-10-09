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
	"fmt"

	"github.com/pkg/errors"

	"github.com/crillab/gophersat/bf"
	pkg "github.com/mudler/luet/pkg/package"
)

type SolverType int

const (
	SingleCoreSimple = 0
)

// PackageSolver is an interface to a generic package solving algorithm
type PackageSolver interface {
	SetDefinitionDatabase(pkg.PackageDatabase)
	Install(p pkg.Packages) (PackagesAssertions, error)
	RelaxedInstall(p pkg.Packages) (PackagesAssertions, error)

	Uninstall(checkconflicts, full bool, candidate ...pkg.Package) (pkg.Packages, error)
	ConflictsWithInstalled(p pkg.Package) (bool, error)
	ConflictsWith(p pkg.Package, ls pkg.Packages) (bool, error)
	Conflicts(pack pkg.Package, lsp pkg.Packages) (bool, error)

	World() pkg.Packages
	Upgrade(checkconflicts, full bool) (pkg.Packages, PackagesAssertions, error)

	UpgradeUniverse(dropremoved bool) (pkg.Packages, PackagesAssertions, error)
	UninstallUniverse(toremove pkg.Packages) (pkg.Packages, error)

	SetResolver(PackageResolver)

	Solve() (PackagesAssertions, error)
	//	BestInstall(c pkg.Packages) (PackagesAssertions, error)
}

// Solver is the default solver for luet
type Solver struct {
	DefinitionDatabase pkg.PackageDatabase
	SolverDatabase     pkg.PackageDatabase
	Wanted             pkg.Packages
	InstalledDatabase  pkg.PackageDatabase

	Resolver PackageResolver
}

type Options struct {
	Type        SolverType `yaml:"type,omitempty"`
	Concurrency int        `yaml:"concurrency,omitempty"`
}

// NewSolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
func NewSolver(t Options, installed pkg.PackageDatabase, definitiondb pkg.PackageDatabase, solverdb pkg.PackageDatabase) PackageSolver {
	return NewResolver(t, installed, definitiondb, solverdb, &Explainer{})
}

// NewResolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
// Using constructors as in the future we foresee warmups for hot-restore solver cache
func NewResolver(t Options, installed pkg.PackageDatabase, definitiondb pkg.PackageDatabase, solverdb pkg.PackageDatabase, re PackageResolver) PackageSolver {
	var s PackageSolver
	switch t.Type {
	default:
		s = &Solver{InstalledDatabase: installed, DefinitionDatabase: definitiondb, SolverDatabase: solverdb, Resolver: re}
	}

	return s
}

// SetDefinitionDatabase is a setter for the definition Database

func (s *Solver) SetDefinitionDatabase(db pkg.PackageDatabase) {
	s.DefinitionDatabase = db
}

// SetResolver is a setter for the unsat resolver backend
func (s *Solver) SetResolver(r PackageResolver) {
	s.Resolver = r
}

func (s *Solver) World() pkg.Packages {
	return s.DefinitionDatabase.World()
}

func (s *Solver) Installed() pkg.Packages {

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

func (s *Solver) noRulesInstalled() bool {
	for _, p := range s.Installed() {
		if len(p.GetConflicts()) != 0 || len(p.GetRequires()) != 0 {
			return false
		}
	}

	return true
}

func (s *Solver) BuildInstalled() (bf.Formula, error) {
	var formulas []bf.Formula
	var packages pkg.Packages
	for _, p := range s.Installed() {
		packages = append(packages, p)
		for _, dep := range p.Related(s.InstalledDatabase) {
			packages = append(packages, dep)
		}
	}

	for _, p := range packages {
		solvable, err := p.BuildFormula(s.InstalledDatabase, s.SolverDatabase)
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

// BuildWorld builds the formula which olds the requirements from the package definitions
// which are available (global state)
func (s *Solver) BuildPartialWorld(includeInstalled bool) (bf.Formula, error) {
	var formulas []bf.Formula
	// NOTE: This block shouldf be enabled in case of very old systems with outdated world sets
	if includeInstalled {
		solvable, err := s.BuildInstalled()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable)
	}

	var packages pkg.Packages
	for _, p := range s.Wanted {
		//	packages = append(packages, p)
		for _, dep := range p.Related(s.DefinitionDatabase) {
			packages = append(packages, dep)
		}

	}

	for _, p := range packages {
		solvable, err := p.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}

	if len(formulas) != 0 {
		return bf.And(formulas...), nil
	}

	return bf.True, nil
}

func (s *Solver) getList(db pkg.PackageDatabase, lsp pkg.Packages) (pkg.Packages, error) {
	var ls pkg.Packages

	for _, pp := range lsp {
		cp, err := db.FindPackage(pp)
		if err != nil {
			packages, err := pp.Expand(db)
			// Expand, and relax search - if not found pick the same one
			if err != nil || len(packages) == 0 {
				cp = pp
			} else {
				cp = packages.Best(nil)
			}
		}
		ls = append(ls, cp)
	}
	return ls, nil
}

// Conflicts acts like ConflictsWith, but uses package's reverse dependencies to
// determine if it conflicts with the given set
func (s *Solver) Conflicts(pack pkg.Package, lsp pkg.Packages) (bool, error) {
	p, err := s.DefinitionDatabase.FindPackage(pack)
	if err != nil {
		p = pack
	}

	ls, err := s.getList(s.DefinitionDatabase, lsp)
	if err != nil {
		return false, errors.Wrap(err, "Package not found in definition db")
	}

	if s.noRulesWorld() {
		return false, nil
	}

	temporarySet := pkg.NewInMemoryDatabase(false)
	for _, p := range ls {
		temporarySet.CreatePackage(p)
	}

	revdeps, err := temporarySet.GetRevdeps(p)
	if err != nil {
		return false, errors.Wrap(err, "error scanning revdeps")
	}

	var revdepsErr error
	for _, r := range revdeps {
		if revdepsErr == nil {
			revdepsErr = errors.New("")
		}
		revdepsErr = fmt.Errorf("%s\n%s", revdepsErr.Error(), r.HumanReadableString())
	}

	return len(revdeps) != 0, revdepsErr
}

// ConflictsWith return true if a package is part of the requirement set of a list of package
// return false otherwise (and thus it is NOT relevant to the given list)
func (s *Solver) ConflictsWith(pack pkg.Package, lsp pkg.Packages) (bool, error) {
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

// UninstallUniverse takes a list of candidate package and return a list of packages that would be removed
// in order to purge the candidate. Uses the solver to check constraints and nothing else
//
// It can be compared to the counterpart Uninstall as this method acts like a uninstall --full
// it removes all the packages and its deps. taking also in consideration other packages that might have
// revdeps
func (s *Solver) UninstallUniverse(toremove pkg.Packages) (pkg.Packages, error) {

	if s.noRulesInstalled() {
		return s.getList(s.InstalledDatabase, toremove)
	}

	// resolve to packages from the db
	toRemove, err := s.getList(s.InstalledDatabase, toremove)
	if err != nil {
		return nil, errors.Wrap(err, "Package not found in definition db")
	}

	var formulas []bf.Formula
	r, err := s.BuildInstalled()
	if err != nil {
		return nil, errors.Wrap(err, "Package not found in definition db")
	}

	// SAT encode the clauses against the world
	for _, p := range toRemove.Unique() {
		encodedP, err := p.Encode(s.InstalledDatabase)
		if err != nil {
			return nil, errors.Wrap(err, "Package not found in definition db")
		}
		P := bf.Var(encodedP)
		formulas = append(formulas, bf.And(bf.Not(P), r))
	}

	markedForRemoval := pkg.Packages{}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return nil, errors.New("Failed finding a solution")
	}
	assertion, err := DecodeModel(model, s.InstalledDatabase)
	if err != nil {
		return nil, errors.Wrap(err, "while decoding model from solution")
	}
	for _, a := range assertion {
		if !a.Value {
			if p, err := s.InstalledDatabase.FindPackage(a.Package); err == nil {
				markedForRemoval = append(markedForRemoval, p)
			}

		}
	}
	return markedForRemoval, nil
}

// UpgradeUniverse mark packages for removal and returns a solution. It considers
// the Universe db as authoritative
// See also on the subject: https://arxiv.org/pdf/1007.1021.pdf
func (s *Solver) UpgradeUniverse(dropremoved bool) (pkg.Packages, PackagesAssertions, error) {
	// we first figure out which aren't up-to-date
	// which has to be removed
	// and which needs to be upgraded
	notUptodate := pkg.Packages{}
	removed := pkg.Packages{}
	toUpgrade := pkg.Packages{}
	replacements := map[pkg.Package]pkg.Package{}

	// TODO: this is memory expensive, we need to optimize this
	universe, err := s.DefinitionDatabase.Copy()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed copying db")
	}

	for _, p := range s.Installed() {
		universe.CreatePackage(p)
	}

	// Grab all the installed ones, see if they are eligible for update
	for _, p := range s.Installed() {
		available, err := s.DefinitionDatabase.FindPackageVersions(p)
		if len(available) == 0 || err != nil {
			removed = append(removed, p)
			continue
		}

		bestmatch := available.Best(nil)
		// Found a better version available
		if !bestmatch.Matches(p) {
			notUptodate = append(notUptodate, p)
			toUpgrade = append(toUpgrade, bestmatch)
			replacements[p] = bestmatch
		}
	}

	var formulas []bf.Formula

	// Build constraints for the whole defdb
	r, err := s.BuildWorld(true)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't build world constraints")
	}

	// Treat removed packages from universe as marked for deletion
	if dropremoved {
		// SAT encode the clauses against the world
		for _, p := range removed.Unique() {
			encodedP, err := p.Encode(universe)
			if err != nil {
				return nil, nil, errors.Wrap(err, "couldn't encode package")
			}
			P := bf.Var(encodedP)
			formulas = append(formulas, bf.And(bf.Not(P), r))
		}
	}

	for old, new := range replacements {
		oldP, err := old.Encode(universe)
		if err != nil {
			return nil, nil, errors.Wrap(err, "couldn't encode package")
		}
		oldencodedP := bf.Var(oldP)
		newP, err := new.Encode(universe)
		if err != nil {
			return nil, nil, errors.Wrap(err, "couldn't encode package")
		}
		newEncodedP := bf.Var(newP)

		//solvable, err := old.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		solvablenew, err := new.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)

		formulas = append(formulas, bf.And(bf.Not(oldencodedP), bf.And(append(solvablenew, newEncodedP)...)))
	}

	//formulas = append(formulas, r)

	markedForRemoval := pkg.Packages{}

	if len(formulas) == 0 {
		return pkg.Packages{}, PackagesAssertions{}, nil
	}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return nil, nil, errors.New("Failed finding a solution")
	}

	assertion, err := DecodeModel(model, universe)
	if err != nil {
		return nil, nil, errors.Wrap(err, "while decoding model from solution")
	}
	for _, a := range assertion {
		if !a.Value {
			if p, err := s.InstalledDatabase.FindPackage(a.Package); err == nil {
				markedForRemoval = append(markedForRemoval, p)
			}

		}

	}
	return markedForRemoval, assertion, nil
}

func inPackage(list []pkg.Package, p pkg.Package) bool {
	for _, l := range list {
		if l.AtomMatches(p) {
			return true
		}
	}
	return false
}

// Compute upgrade between packages if specified, or all if none is specified
func (s *Solver) computeUpgrade(pps ...pkg.Package) func(defDB pkg.PackageDatabase, installDB pkg.PackageDatabase) (pkg.Packages, pkg.Packages, pkg.PackageDatabase) {
	return func(defDB pkg.PackageDatabase, installDB pkg.PackageDatabase) (pkg.Packages, pkg.Packages, pkg.PackageDatabase) {
		toUninstall := pkg.Packages{}
		toInstall := pkg.Packages{}

		// we do this in memory so we take into account of provides, and its faster
		universe, _ := defDB.Copy()

		installedcopy := pkg.NewInMemoryDatabase(false)

		for _, p := range installDB.World() {
			installedcopy.CreatePackage(p)
			packages, err := universe.FindPackageVersions(p)
			if err == nil && len(packages) != 0 {
				best := packages.Best(nil)
				if !best.Matches(p) && len(pps) == 0 ||
					len(pps) != 0 && inPackage(pps, p) {
					toUninstall = append(toUninstall, p)
					toInstall = append(toInstall, best)
				}
			}
		}
		return toUninstall, toInstall, installedcopy
	}
}

func (s *Solver) upgrade(fn func(defDB pkg.PackageDatabase, installDB pkg.PackageDatabase) (pkg.Packages, pkg.Packages, pkg.PackageDatabase), defDB pkg.PackageDatabase, installDB pkg.PackageDatabase, checkconflicts, full bool) (pkg.Packages, PackagesAssertions, error) {

	toUninstall, toInstall, installedcopy := fn(defDB, installDB)

	s2 := NewSolver(Options{Type: SingleCoreSimple}, installedcopy, defDB, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)
	if !full {
		ass := PackagesAssertions{}
		for _, i := range toInstall {
			ass = append(ass, PackageAssert{Package: i.(*pkg.DefaultPackage), Value: true})
		}
	}
	// Then try to uninstall the versions in the system, and store that tree
	r, err := s.Uninstall(checkconflicts, false, toUninstall.Unique()...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't uninstall candidates ")
	}
	for _, z := range r {
		err = installedcopy.RemovePackage(z)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't remove copy of package targetted for removal")
		}
	}

	if len(toInstall) == 0 {
		ass := PackagesAssertions{}
		for _, i := range installDB.World() {
			ass = append(ass, PackageAssert{Package: i.(*pkg.DefaultPackage), Value: true})
		}
		return toUninstall, ass, nil
	}

	assertions, err := s2.RelaxedInstall(toInstall.Unique())

	return toUninstall, assertions, err
}

func (s *Solver) Upgrade(checkconflicts, full bool) (pkg.Packages, PackagesAssertions, error) {
	return s.upgrade(s.computeUpgrade(), s.DefinitionDatabase, s.InstalledDatabase, checkconflicts, full)
}

// Uninstall takes a candidate package and return a list of packages that would be removed
// in order to purge the candidate. Returns error if unsat.
func (s *Solver) Uninstall(checkconflicts, full bool, packs ...pkg.Package) (pkg.Packages, error) {
	if len(packs) == 0 {
		return pkg.Packages{}, nil
	}
	var res pkg.Packages

	toRemove := pkg.Packages{}

	for _, c := range packs {
		candidate, err := s.InstalledDatabase.FindPackage(c)
		if err != nil {

			//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
			packages, err := c.Expand(s.InstalledDatabase)
			//	Info("Expanded", packages, err)
			if err != nil || len(packages) == 0 {
				candidate = c
			} else {
				candidate = packages.Best(nil)
			}
			//Relax search, otherwise we cannot compute solutions for packages not in definitions
			//	return nil, errors.Wrap(err, "Package not found between installed")
		}

		toRemove = append(toRemove, candidate)
	}

	// Build a fake "Installed" - Candidate and its requires tree
	var InstalledMinusCandidate pkg.Packages

	// We are asked to not perform a full uninstall (checking all the possible requires that could
	// be removed). Let's only check if we can remove the selected package
	if !full && checkconflicts {
		for _, candidate := range toRemove {
			if conflicts, err := s.Conflicts(candidate, s.Installed()); conflicts {
				return nil, errors.Wrap(err, "while searching for "+candidate.HumanReadableString()+" conflicts")
			}
		}
		return toRemove, nil
	}

	// TODO: Can be optimized
	for _, i := range s.Installed() {
		matched := false
		for _, candidate := range toRemove {
			if !i.Matches(candidate) {
				contains, err := candidate.RequiresContains(s.SolverDatabase, i)
				if err != nil {
					return nil, errors.Wrap(err, "Failed getting installed list")
				}
				if !contains {
					matched = true
				}

			}
		}
		if matched {
			InstalledMinusCandidate = append(InstalledMinusCandidate, i)
		}
	}

	s2 := NewSolver(Options{Type: SingleCoreSimple}, pkg.NewInMemoryDatabase(false), s.InstalledDatabase, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)

	// Get the requirements to install the candidate
	asserts, err := s2.RelaxedInstall(toRemove)
	if err != nil {
		return nil, err
	}
	for _, a := range asserts {
		if a.Value {
			if !checkconflicts {
				res = append(res, a.Package)
				continue
			}

			c, err := s.ConflictsWithInstalled(a.Package)
			if err != nil {
				return nil, err
			}

			// If doesn't conflict with installed we just consider it for removal and look for the next one
			if !c {
				res = append(res, a.Package)
				continue
			}

			// If does conflicts, give it another chance by checking conflicts if in case we didn't installed our candidate and all the required packages in the system
			c, err = s.ConflictsWith(a.Package, InstalledMinusCandidate)
			if err != nil {
				return nil, err
			}
			if !c {
				res = append(res, a.Package)
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
		//	allW = append(allW, W)
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
func (s *Solver) RelaxedInstall(c pkg.Packages) (PackagesAssertions, error) {

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
	assertions, err := s.Solve()
	if err != nil {
		return nil, err
	}

	return assertions, nil
}

// Install returns the assertions necessary in order to install the packages in
// a system.
// It calculates the best result possible, trying to maximize new packages.
func (s *Solver) Install(c pkg.Packages) (PackagesAssertions, error) {
	assertions, err := s.RelaxedInstall(c)
	if err != nil {
		return nil, err
	}

	systemAfterInstall := pkg.NewInMemoryDatabase(false)

	toUpgrade := pkg.Packages{}

	for _, p := range c {
		if p.GetVersion() == ">=0" || p.GetVersion() == ">0" {
			toUpgrade = append(toUpgrade, p)
		}
	}
	for _, p := range assertions {
		if p.Value {
			systemAfterInstall.CreatePackage(p.Package)
			if !inPackage(c, p.Package) {
				toUpgrade = append(toUpgrade, p.Package)
			}
		}
	}

	if len(toUpgrade) == 0 {
		return assertions, nil
	}
	// do partial upgrade based on input.
	// IF there is no version specified in the input, or >=0 is specified,
	// then compute upgrade for those
	_, newassertions, err := s.upgrade(s.computeUpgrade(toUpgrade...), s.DefinitionDatabase, systemAfterInstall, false, false)
	if err != nil {
		// TODO: Emit warning.
		// We were not able to compute upgrades (maybe for some pinned packages, or a conflict)
		// so we return the relaxed result
		return assertions, nil
	}

	// Protect if we return no assertion at all
	if len(newassertions) == 0 && len(assertions) > 0 {
		return assertions, nil
	}

	return newassertions, nil
}
