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
	pkg "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/solver"
)

var _ = Describe("Solver", func() {

	Context("Simple set", func() {
		It("Solves correctly if the selected package has no requirements or conflicts and we have nothing installed yet", func() {
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{}, []pkg.Package{A, B, C})
			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("Solves correctly if the selected package has no requirements or conflicts and we have installed one package", func() {
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C})
			solution, err := s.Install([]pkg.Package{B})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(2))
		})

		It("Solves correctly if the selected package to install has no requirement or conflicts, but in the world there is one with a requirement", func() {
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{E, C}, []pkg.Package{A, B, C, D, E})
			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: E.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: false}))

			Expect(len(solution)).To(Equal(5))
		})

		It("Solves correctly if the selected package to install has requirements", func() {
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C, D})
			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))

			Expect(len(solution)).To(Equal(3))
		})

		It("Solves correctly", func() {
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C})
			solution, err := s.Install([]pkg.Package{A})
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

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C, D})

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves correctly more complex ones", func() {
			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{}, []pkg.Package{A, B, C, D, E})

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Uninstalls simple package correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})

			solution, err := s.Uninstall(A)
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(A.IsFlagged(false)))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("Find conflicts", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})
			val, err := s.ConflictsWithInstalled(A)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())

		})

		It("Find nested conflicts", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})
			val, err := s.ConflictsWithInstalled(D)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())
		})

		It("Doesn't find nested conflicts", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})
			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Doesn't find conflicts", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})
			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})
		It("Uninstalls simple packages not in world correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{B, C, D})

			solution, err := s.Uninstall(A)
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(A.IsFlagged(false)))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("Uninstalls complex packages not in world correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{B, C, D})

			solution, err := s.Uninstall(A)
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(A.IsFlagged(false)))

			Expect(len(solution)).To(Equal(1))
		})

		It("Uninstalls complex packages correctly, even if shared deps are required by system packages", func() {
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})

			solution, err := s.Uninstall(A)
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(A.IsFlagged(false)))
			Expect(solution).ToNot(ContainElement(B.IsFlagged(false)))

			Expect(len(solution)).To(Equal(1))
		})

		It("Uninstalls complex packages in world correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{C}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{A, C, D}, []pkg.Package{A, B, C, D})

			solution, err := s.Uninstall(A)
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(A.IsFlagged(false)))
			Expect(solution).To(ContainElement(C.IsFlagged(false)))

			Expect(len(solution)).To(Equal(2))
		})

		It("Uninstalls complex package correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C.IsFlagged(true) // installed

			s := NewSolver([]pkg.Package{A, B, C, D}, []pkg.Package{A, B, C, D})

			solution, err := s.Uninstall(A)
			Expect(solution).To(ContainElement(A.IsFlagged(false)))
			Expect(solution).To(ContainElement(B.IsFlagged(false)))
			Expect(solution).To(ContainElement(D.IsFlagged(false)))

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

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C})

			solution, err := s.Install([]pkg.Package{A})
			Expect(len(solution)).To(Equal(0))
			Expect(err).To(HaveOccurred())
		})

	})

	Context("Complex data sets", func() {
		It("Solves them correctly", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			F := pkg.NewPackage("F", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			G := pkg.NewPackage("G", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			H := pkg.NewPackage("H", "", []*pkg.DefaultPackage{G}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{H}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			s := NewSolver([]pkg.Package{C}, []pkg.Package{A, B, C, D, E, F, G})

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: H.IsFlagged(true), Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: G.IsFlagged(true), Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})
	})

})
