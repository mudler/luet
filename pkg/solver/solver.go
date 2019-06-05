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

	"github.com/crillab/gophersat/bf"
	pkg "gitlab.com/mudler/luet/pkg/package"
)

type State interface{ Encode() string }

type PackageSolver interface {
	SetWorld(p []pkg.Package)
	Install(p []pkg.Package) ([]PackageAssert, error)
	Uninstall(candidate pkg.Package) ([]pkg.Package, error)
}
type Solver struct {
	Wanted    []pkg.Package
	Installed []pkg.Package
	World     []pkg.Package
}

func NewSolver(init []pkg.Package, w []pkg.Package) PackageSolver {
	for _, v := range init {
		pkg.NormalizeFlagged(v)
	}
	for _, v := range w {
		pkg.NormalizeFlagged(v)
	}
	return &Solver{Installed: init, World: w}
}

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

func (s *Solver) BuildWorld() (bf.Formula, error) {
	var formulas []bf.Formula
	// for _, p := range s.Installed {
	// 	solvable, err := p.BuildFormula()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	//f = bf.And(f, solvable)
	// 	formulas = append(formulas, solvable...)

	// }
	for _, p := range s.World {
		solvable, err := p.BuildFormula()
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}
	return bf.And(formulas...), nil
}

// world is ok with Px (installed-x-th) and removal of package (candidate?)
// collect unsatisfieds and repeat until we get no more unsatisfieds
func (s *Solver) Uninstall(candidate pkg.Package) ([]pkg.Package, error) {
	var res []pkg.Package
	saved := s.Installed
	s.Installed = []pkg.Package{}

	asserts, err := s.Install([]pkg.Package{candidate})
	if err != nil {
		return nil, err
	}
	s.Installed = saved

	for _, a := range asserts {
		if a.Value && a.Package.Flagged() {
			res = append(res, a.Package.IsFlagged(false))
		}

	}

	return res, nil
}

func (s *Solver) BuildFormula() (bf.Formula, error) {
	//f := bf.True
	var formulas []bf.Formula
	r, err := s.BuildWorld()
	if err != nil {
		return nil, err
	}
	for _, wanted := range s.Wanted {
		encodedW, err := wanted.Encode()
		if err != nil {
			return nil, err
		}
		W := bf.Var(encodedW)

		if len(s.Installed) == 0 {
			formulas = append(formulas, W) //bf.And(bf.True, W))
			continue
		}

		for _, installed := range s.Installed {
			encodedI, err := installed.Encode()
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

func (s *Solver) Solve() ([]PackageAssert, error) {

	f, err := s.BuildFormula()

	if err != nil {
		return nil, err
	}

	model, _, err := s.solve(f)
	if err != nil {
		return nil, err
	}

	return DecodeModel(model)
}

func (s *Solver) Install(coll []pkg.Package) ([]PackageAssert, error) {
	for _, v := range coll {
		v.IsFlagged(false)
	}
	s.Wanted = coll

	if s.noRulesWorld() {
		var ass []PackageAssert
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
