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
}
type Solver struct {
	Wanted    []pkg.Package
	Installed []pkg.Package
	World     []pkg.Package
}

func NewSolver(init []pkg.Package, w []pkg.Package) PackageSolver {
	for _, v := range init {
		v.IsFlagged(true)
	}
	for _, v := range w {
		v.IsFlagged(true)
	}
	return &Solver{Installed: init, World: w}
}

func (s *Solver) SetWorld(p []pkg.Package) {
	s.World = p
}

func (s *Solver) BuildWorld() (bf.Formula, error) {
	var formulas []bf.Formula

	for _, p := range s.Wanted {
		solvable, err := p.BuildFormula()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable...)

	}
	return bf.And(formulas...), nil
}

func (s *Solver) BuildFormula() (bf.Formula, error) {
	//f := bf.True
	var formulas []bf.Formula
	r, err := s.BuildWorld()
	if err != nil {
		return nil, err
	}
	formulas = append(formulas, r)
	for _, wanted := range s.Wanted {
		encodedW, err := wanted.IsFlagged(true).Encode()
		if err != nil {
			return nil, err
		}
		W := bf.Var(encodedW)
		if len(s.Installed) == 0 {
			formulas = append(formulas, bf.And(bf.True, W))
			continue
		}
		for _, installed := range s.Installed {
			encodedI, err := installed.IsFlagged(true).Encode()
			if err != nil {
				return nil, err
			}
			I := bf.Var(encodedI)
			formulas = append(formulas, bf.And(W, I))
		}

	}
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

	return s.Solve()
}
