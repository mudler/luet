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

package solver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pkg "gitlab.com/mudler/luet/pkg/package"

	. "gitlab.com/mudler/luet/pkg/solver"
)

var _ = Describe("Solver", func() {

	Context("Simple set", func() {

		It("Solves correctly", func() {
			B := pkg.NewPackage("B", "", []pkg.Package{}, []pkg.Package{})
			A := pkg.NewPackage("A", "", []pkg.Package{B}, []pkg.Package{})
			C := pkg.NewPackage("C", "", []pkg.Package{}, []pkg.Package{})
			C.IsFlagged(true) // installed

			s := NewSolver([]pkg.Package{A.IsFlagged(true)}, []pkg.Package{C}) // XXX: goes fatal with odd numbers of cnf ?

			solution, err := s.Solve()
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(B.IsFlagged(true)))
			Expect(solution).To(ContainElement(C.IsFlagged(true)))
			// Expect(solution).To(ContainElement(A.IsFlagged(true)))
		})
	})

	Context("Conflict set", func() {

		It("Solves correctly", func() {
			C := pkg.NewPackage("C", "", []pkg.Package{}, []pkg.Package{})
			//	D := pkg.NewPackage("D", "", []pkg.Package{}, []pkg.Package{})
			B := pkg.NewPackage("B", "", []pkg.Package{}, []pkg.Package{C})
			A := pkg.NewPackage("A", "", []pkg.Package{B}, []pkg.Package{})
			C.IsFlagged(true) // installed

			s := NewSolver([]pkg.Package{A.IsFlagged(true)}, []pkg.Package{C})

			solution, err := s.Solve()
			Expect(solution).To(Equal([]pkg.Package{C}))

			Expect(err).ToNot(HaveOccurred())
		})

	})

})
