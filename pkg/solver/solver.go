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
	Solve() ([]PackageAssert, error)
	Apply() (map[string]bool, bf.Formula, error)
	SetSteps(int)
	SetWorld(p []pkg.Package)
}
type Solver struct {
	Wanted    []pkg.Package
	Installed []pkg.Package
	World     []pkg.Package
	Steps     int
}

func NewSolver(pcoll []pkg.Package, init []pkg.Package, w []pkg.Package) PackageSolver {
	for _, v := range init {
		v.IsFlagged(true)
	}
	for _, v := range pcoll {
		v.IsFlagged(false)
	}
	for _, v := range w {
		v.IsFlagged(true)
	}
	return &Solver{Wanted: pcoll, Installed: init, World: w}
}

func (s *Solver) SetSteps(st int) {
	s.Steps = st
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
		for _, installed := range s.Installed {
			encodedI, err := installed.IsFlagged(true).Encode()
			if err != nil {
				return nil, err
			}
			I := bf.Var(encodedI)
			formulas = append(formulas, bf.And(W, I))
		}

	}
	//return bf.And(r), nil
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
	fmt.Println("Steps", s.Steps, f)
	if err != nil {
		return map[string]bool{}, nil, err
	}

	// if s.Steps != 0 {
	// 	for i := s.Steps; i >= 0; i-- {
	// 		f, err = s.BuildFormula()
	// 		if err != nil {
	// 			return map[string]bool{}, nil, err
	// 		}
	// 		model, _, err := s.solve(f)
	// 		if err != nil {
	// 			return map[string]bool{}, nil, err
	// 		}
	// 		fmt.Println("Step ", i, model)

	// 		ass := DecodeModel(model)
	// 		s.Installed = append(s.Installed, ass...)
	// 	}
	// }

	return s.solve(f)
}

func (s *Solver) Solve() ([]PackageAssert, error) {
	model, _, err := s.Apply()
	if err != nil {
		return nil, err
	}

	// for _, wanted := range s.Wanted {
	// 	encodedW, err := wanted.Encode()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	model[encodedW] = true
	// 	fmt.Println("adding wanted", model)

	// }
	return DecodeModel(model)
}
