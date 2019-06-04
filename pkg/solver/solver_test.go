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
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A}, []pkg.Package{C}, []pkg.Package{A, B, C}) // XXX: goes fatal with odd numbers of cnf ?
			solution, err := s.Solve()
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(3))
		})
		It("Solves correctly more complex ones", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C.IsFlagged(true) // installed

			s := NewSolver([]pkg.Package{A.IsFlagged(true)}, []pkg.Package{C}, []pkg.Package{A, B, C, D})

			solution, err := s.Solve()
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves correctly more complex ones", func() {
			//	E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A}, []pkg.Package{}, []pkg.Package{A, B, C, D})

			solution, err := s.Solve()
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

	})

	Context("Conflict set", func() {

		It("is unsolvable - as we something we ask to install conflict with system stuff", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			//	D := pkg.NewPackage("D", "", []pkg.Package{}, []pkg.Package{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A}, []pkg.Package{C}, []pkg.Package{A, B, C})

			solution, err := s.Solve()
			Expect(len(solution)).To(Equal(0))
			Expect(err).To(HaveOccurred())
		})

	})

})
