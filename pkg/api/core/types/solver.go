// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types

import (
	"github.com/crillab/gophersat/bf"
)

type SolverType int

const (
	SolverSingleCoreSimple SolverType = 0
)

// PackageSolver is an interface to a generic package solving algorithm
type PackageSolver interface {
	SetDefinitionDatabase(PackageDatabase)
	Install(p Packages) (PackagesAssertions, error)
	RelaxedInstall(p Packages) (PackagesAssertions, error)

	Uninstall(checkconflicts, full bool, candidate ...*Package) (Packages, error)
	ConflictsWithInstalled(p *Package) (bool, error)
	ConflictsWith(p *Package, ls Packages) (bool, error)
	Conflicts(pack *Package, lsp Packages) (bool, error)

	World() Packages
	Upgrade(checkconflicts, full bool) (Packages, PackagesAssertions, error)

	UpgradeUniverse(dropremoved bool) (Packages, PackagesAssertions, error)
	UninstallUniverse(toremove Packages) (Packages, error)

	SetResolver(PackageResolver)

	Solve() (PackagesAssertions, error)
	//	BestInstall(c Packages) (PackagesAssertions, error)
}

type SolverOptions struct {
	Type        SolverType `yaml:"type,omitempty"`
	Concurrency int        `yaml:"concurrency,omitempty"`

	// Optimize enables an EXPERIMENTAL post-solve pass that tries to pull each
	// package up to the newest version that is still satisfiable.
	//
	// Without it, "newest" is not something the solver is asked for. The
	// encoding is version-blind - versions of a package are opaque, mutually
	// exclusive atoms with no ordering between them - so preferring the newest
	// relies on presenting candidates newest-first and on gophersat happening to
	// reach the first one. That holds in most cases but is a branching-order
	// side effect, not a guarantee: once clause learning starts driving variable
	// activity, an older version can be decided first. Measured on generated
	// worlds, roughly 16% ended up on a version that a re-solve proved was
	// needlessly old.
	//
	// The pass costs one extra SAT solve per attempted improvement, which is why
	// it is opt-in.
	Optimize bool `yaml:"optimize,omitempty"`
}

// PackageResolver assists PackageSolver on unsat cases
type PackageResolver interface {
	Solve(bf.Formula, PackageSolver) (PackagesAssertions, error)
}

type PackagesAssertions []PackageAssert

type PackageHash struct {
	BuildHash   string
	PackageHash string
}
