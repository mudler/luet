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
	"fmt"

	"github.com/crillab/gophersat/bf"
	pkg "gitlab.com/mudler/luet/pkg/package"
)

type State interface{ Encode() string }

type PackageSolver interface {
	BuildFormula() (bf.Formula, error)
	Solve() ([]pkg.Package, error)
	Apply() (map[string]bool, bf.Formula, error)
}
type Solver struct {
	PackageCollection []pkg.Package
	InitialState      []pkg.Package
}

func NewSolver(pcoll []pkg.Package, init []pkg.Package) PackageSolver {
	return &Solver{PackageCollection: pcoll, InitialState: init}
}

func (s *Solver) BuildFormula() (bf.Formula, error) {
	//f := bf.True
	var formulas []bf.Formula

	for _, a := range s.InitialState {
		init, err := a.BuildFormula()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, init)
		formulas = append(formulas, init...)
	}

	for _, p := range s.PackageCollection {
		solvable, err := p.BuildFormula()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable...)

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

func (s *Solver) Apply() (map[string]bool, bf.Formula, error) {
	f, err := s.BuildFormula()
	fmt.Println(f)
	if err != nil {
		return map[string]bool{}, nil, err
	}
	return s.solve(f)
}

func (s *Solver) Solve() ([]pkg.Package, error) {
	model, _, err := s.Apply()
	if err != nil {
		return []pkg.Package{}, err
	}
	ass := DecodeModel(model)
	return ass, nil
}
