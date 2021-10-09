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
	"fmt"

	pkg "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/solver"
)

var _ = Describe("Solver", func() {

	db := pkg.NewInMemoryDatabase(false)
	dbInstalled := pkg.NewInMemoryDatabase(false)
	dbDefinitions := pkg.NewInMemoryDatabase(false)
	s := NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

	BeforeEach(func() {
		db = pkg.NewInMemoryDatabase(false)
		dbInstalled = pkg.NewInMemoryDatabase(false)
		dbDefinitions = pkg.NewInMemoryDatabase(false)
		s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)
	})

	Context("Select of best available package", func() {

		It("picks the best versions available for each package, excluding the ones manually specified while installing", func() {

			B1 := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B2 := pkg.NewPackage("B", "1.2", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B3 := pkg.NewPackage("B", "1.3", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B4 := pkg.NewPackage("B", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			A1 := pkg.NewPackage("A", "1.1", []*pkg.DefaultPackage{
				pkg.NewPackage("B", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})
			A2 := pkg.NewPackage("A", "1.2", []*pkg.DefaultPackage{
				pkg.NewPackage("B", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})

			D := pkg.NewPackage("D", "1.0", []*pkg.DefaultPackage{
				pkg.NewPackage("A", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})

			C := pkg.NewPackage("C", "1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A1, A2, B1, B2, B3, B4, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.(*Solver).Install([]pkg.Package{D})
			fmt.Println(solution)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(solution)).To(Equal(8))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B4, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B2, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B3, Value: false}))

			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err = s.(*Solver).Install([]pkg.Package{D, B2})
			fmt.Println(solution)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(solution)).To(Equal(8))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B2, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B4, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B3, Value: false}))

		})

		It("picks the best available excluding those manually input. In this case we the input is a selector >=0", func() {

			B1 := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B2 := pkg.NewPackage("B", "1.2", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B3 := pkg.NewPackage("B", "1.3", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B4 := pkg.NewPackage("B", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			A1 := pkg.NewPackage("A", "1.1", []*pkg.DefaultPackage{
				pkg.NewPackage("B", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})
			A2 := pkg.NewPackage("A", "1.2", []*pkg.DefaultPackage{
				pkg.NewPackage("B", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})

			D := pkg.NewPackage("D", "1.0", []*pkg.DefaultPackage{
				pkg.NewPackage("A", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}),
			}, []*pkg.DefaultPackage{})

			C := pkg.NewPackage("C", "1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A1, A2, B1, B2, B3, B4, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.(*Solver).Install([]pkg.Package{pkg.NewPackage("D", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})})
			fmt.Println(solution)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(solution)).To(Equal(8))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B4, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B2, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B3, Value: false}))

			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err = s.(*Solver).Install([]pkg.Package{pkg.NewPackage("D", ">=0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{}), B2})
			fmt.Println(solution)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(solution)).To(Equal(8))

			//	Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B2, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B1, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B4, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B3, Value: false}))

		})
	})
	Context("Simple set", func() {
		It("Solves correctly if the selected package has no requirements or conflicts and we have nothing installed yet", func() {

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("Solves correctly if the selected package has no requirements or conflicts and we have installed one package", func() {

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{B})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(2))
		})

		It("Solves correctly if the selected package to install has no requirement or conflicts, but in the world there is one with a requirement", func() {

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{E, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: E, Value: true}))
			//	Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: false}))
			//Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))

			Expect(len(solution)).To(Equal(5))
		})

		It("Solves correctly if the selected package to install has requirements", func() {

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))

			Expect(len(solution)).To(Equal(3))
		})

		It("Solves correctly", func() {

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
		})
		It("Solves correctly more complex ones", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves correctly more complex ones", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves deps with expansion", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "B", Version: ">1.0"}}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Solves deps with more expansion", func() {

			C := pkg.NewPackage("c", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "a", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			C.SetCategory("test")
			B := pkg.NewPackage("b", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B.SetCategory("test")
			A := pkg.NewPackage("a", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A.SetCategory("test")

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{C})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves deps with more expansion", func() {

			C := pkg.NewPackage("c", "1.5", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "a", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			C.SetCategory("test")
			B := pkg.NewPackage("b", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B.SetCategory("test")
			A := pkg.NewPackage("a", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A.SetCategory("test")

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{&pkg.DefaultPackage{Name: "c", Version: ">1.0", Category: "test"}})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Solves deps with more expansion", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "B", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Selects one version", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D2 := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "1.8", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D1 := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: "1.4"}}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, D1, D2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A, B})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D2, Value: false}))

			Expect(len(solution)).To(Equal(5))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Install only package requires", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C := pkg.NewPackage("C", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{
				Name:    "A",
				Version: ">=1.0",
			}}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{
				&pkg.DefaultPackage{
					Name:    "D",
					Version: ">=0",
				},
			}, []*pkg.DefaultPackage{})

			A := pkg.NewPackage("A", "1.2", []*pkg.DefaultPackage{
				&pkg.DefaultPackage{
					Name:    "D",
					Version: ">=1.0",
				},
			}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{C})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Selects best version", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			E.SetCategory("test")
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C.SetCategory("test")
			D2 := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D2.SetCategory("test")
			D := pkg.NewPackage("D", "1.8", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D.SetCategory("test")
			D1 := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D1.SetCategory("test")
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			B.SetCategory("test")
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A.SetCategory("test")

			for _, p := range []pkg.Package{A, B, C, D, D1, D2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A, B})
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D1, Value: false}))

			Expect(len(solution)).To(Equal(5))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides", func() {

			E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			E.SetCategory("test")
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			C.SetCategory("test")
			D2 := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D2.SetCategory("test")
			D := pkg.NewPackage("D", "1.8", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D.SetCategory("test")
			D1 := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D1.SetCategory("test")
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			B.SetCategory("test")
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A.SetCategory("test")

			D2.SetProvides([]*pkg.DefaultPackage{{Name: "E", Category: "test"}})
			A2 := pkg.NewPackage("A", "1.3", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "E", Version: "", Category: "test"}}, []*pkg.DefaultPackage{})
			A2.SetCategory("test")

			for _, p := range []pkg.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A2, B})
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides with versions", func() {
			E := pkg.NewPackage("E", "1.3", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D2 := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "1.8", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D1 := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			D2.SetProvides([]*pkg.DefaultPackage{{Name: "E", Version: "1.3"}})
			A2 := pkg.NewPackage("A", "1.3", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "E", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A2})
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: A, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides with selectors", func() {
			E := pkg.NewPackage("E", "1.3", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D2 := pkg.NewPackage("D", "1.9", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "1.8", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D1 := pkg.NewPackage("D", "1.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B := pkg.NewPackage("B", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "D", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			D2.SetProvides([]*pkg.DefaultPackage{{Name: "E", Version: ">=1.3"}})
			A2 := pkg.NewPackage("A", "1.3", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "E", Version: ">=1.0"}}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]pkg.Package{A2})
			Expect(solution).To(ContainElement(PackageAssert{Package: A2, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: A, Value: true}))

			Expect(solution).To(ContainElement(PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("Uninstall", func() {
			It("Uninstalls simple package correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("Uninstalls simple package expanded correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "1.2", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.Uninstall(true, true, &pkg.DefaultPackage{Name: "A", Version: ">1.0"})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("Uninstalls simple packages not in world correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})

			It("Uninstalls complex packages not in world correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))

				Expect(len(solution)).To(Equal(2))
			})

			It("Uninstalls complex packages correctly, even if shared deps are required by system packages", func() {
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).ToNot(ContainElement(B))

				Expect(len(solution)).To(Equal(1))
			})

			It("Uninstalls complex packages in world correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{C}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(C))

				Expect(len(solution)).To(Equal(2))
			})

			It("Uninstalls complex package correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				//	C // installed

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))
				Expect(solution).To(ContainElement(D))

				Expect(len(solution)).To(Equal(3))

			})

			It("Fails to uninstall if a package is required", func() {
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				Z := pkg.NewPackage("Z", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})
				F := pkg.NewPackage("F", "", []*pkg.DefaultPackage{Z, B}, []*pkg.DefaultPackage{})

				Z.SetVersion("1.4101.dvw.dqc.")
				B.SetVersion("1.4101qe.eq.ff..dvw.dqc.")
				C.SetVersion("1.aaaa.eq.ff..dvw.dqc.")

				for _, p := range []pkg.Package{A, B, C, D, Z, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D, Z, F} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Uninstall(true, false, B)
				Expect(err).To(HaveOccurred())
				Expect(len(solution)).To(Equal(0))
			})

			It("UninstallUniverse simple package correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("UninstallUniverse simple package expanded correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "1.2", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(Options{Type: SingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.UninstallUniverse(pkg.Packages{
					&pkg.DefaultPackage{Name: "A", Version: ">1.0"}})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("UninstallUniverse simple packages not in world correctly", func() {

				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})

			It("UninstallUniverse complex packages not in world correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))

				Expect(len(solution)).To(Equal(2))
			})

			It("UninstallUniverse complex packages correctly, even if shared deps are required by system packages", func() {
				// Here we diff a lot from standard Uninstall:
				// all the packages that has reverse deps will be removed (aka --full)
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))
				Expect(solution).To(ContainElement(C))

				Expect(len(solution)).To(Equal(3))
			})

			It("UninstallUniverse complex packages in world correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{C}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(C))

				Expect(len(solution)).To(Equal(2))
			})

			It("UninstallUniverse complex package correctly", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				//	C // installed

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.UninstallUniverse(pkg.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))
				Expect(solution).To(ContainElement(D))

				Expect(len(solution)).To(Equal(3))

			})

		})
		It("Find conflicts", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(A)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())

		})

		It("Find nested conflicts", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(D)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())
		})

		It("Doesn't find nested conflicts", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Doesn't find conflicts", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Find conflicts using revdeps", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(A, dbInstalled.World())
			Expect(err.Error()).To(Equal("\n/B-"))
			Expect(val).To(BeTrue())

		})

		It("Find nested conflicts with revdeps", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(D, dbInstalled.World())
			Expect(err.Error()).To(Or(Equal("\n/A-\n/B-"), Equal("\n/B-\n/A-")))
			Expect(val).To(BeTrue())
		})

		It("Doesn't find nested conflicts with revdeps", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{D}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{A}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(C, dbInstalled.World())
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Doesn't find conflicts with revdeps", func() {

			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			val, err := s.Conflicts(C, dbInstalled.World())
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

	})

	Context("Conflict set", func() {

		It("is unsolvable - as we something we ask to install conflict with system stuff", func() {
			C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			//	D := pkg.NewPackage("D", "", []pkg.Package{}, []pkg.Package{})
			B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
			A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})

			for _, p := range []pkg.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

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

			for _, p := range []pkg.Package{A, B, C, D, E, F, G} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			solution, err := s.Install([]pkg.Package{A})
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: H, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: G, Value: true}))

			Expect(len(solution)).To(Equal(6))
		})
	})

	Context("Selection", func() {
		a := pkg.NewPackage("A", ">=2.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		a1 := pkg.NewPackage("A", "2.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		a11 := pkg.NewPackage("A", "2.1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		a01 := pkg.NewPackage("A", "2.2", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		a02 := pkg.NewPackage("A", "2.3", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		a03 := pkg.NewPackage("A", "2.3.4", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		old := pkg.NewPackage("A", "1.3.1", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

		It("Expands correctly", func() {
			definitions := pkg.NewInMemoryDatabase(false)
			for _, p := range []pkg.Package{a1, a11, a01, a02, a03, old} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst, err := a.Expand(definitions)
			Expect(err).ToNot(HaveOccurred())
			Expect(lst).To(ContainElement(a11))
			Expect(lst).To(ContainElement(a1))
			Expect(lst).To(ContainElement(a01))
			Expect(lst).To(ContainElement(a02))
			Expect(lst).To(ContainElement(a03))
			Expect(lst).ToNot(ContainElement(old))
			Expect(len(lst)).To(Equal(5))
			p := lst.Best(nil)
			Expect(p).To(Equal(a03))
		})
	})
	Context("Upgrades", func() {
		E := pkg.NewPackage("e", "1.5", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		E.SetCategory("test")
		C := pkg.NewPackage("c", "1.5", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "a", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
		C.SetCategory("test")
		B := pkg.NewPackage("b", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		B.SetCategory("test")
		A := pkg.NewPackage("a", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
		A.SetCategory("test")
		A1 := pkg.NewPackage("a", "1.2", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
		A1.SetCategory("test")

		BeforeEach(func() {
			C = pkg.NewPackage("c", "1.5", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "a", Version: ">=1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			C.SetCategory("test")
			B = pkg.NewPackage("b", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
			B.SetCategory("test")
			A = pkg.NewPackage("a", "1.1", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A.SetCategory("test")
			A1 = pkg.NewPackage("a", "1.2", []*pkg.DefaultPackage{&pkg.DefaultPackage{Name: "b", Version: "1.0", Category: "test"}}, []*pkg.DefaultPackage{})
			A1.SetCategory("test")
		})

		It("upgrades correctly", func() {
			for _, p := range []pkg.Package{A1, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(1))
			Expect(uninstall[0].GetName()).To(Equal("a"))
			Expect(uninstall[0].GetVersion()).To(Equal("1.1"))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: false}))
			Expect(len(solution)).To(Equal(3))
		})

		It("upgrades correctly with provides", func() {
			B.SetProvides([]*pkg.DefaultPackage{
				&pkg.DefaultPackage{Name: "a", Version: ">=0", Category: "test"},
				&pkg.DefaultPackage{Name: "c", Version: ">=0", Category: "test"},
			})

			for _, p := range []pkg.Package{B} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(2))

			Expect(uninstall).To(ContainElement(C))
			Expect(uninstall).To(ContainElement(A))

			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		PIt("upgrades correctly with provides, also if definitiondb contains both a provide, and the package to be provided", func() {
			B.SetProvides([]*pkg.DefaultPackage{
				&pkg.DefaultPackage{Name: "a", Version: ">=0", Category: "test"},
				&pkg.DefaultPackage{Name: "c", Version: ">=0", Category: "test"},
			})

			for _, p := range []pkg.Package{A1, B} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(2))

			Expect(uninstall).To(ContainElement(C))
			Expect(uninstall).To(ContainElement(A))

			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("UpgradeUniverse upgrades correctly", func() {
			for _, p := range []pkg.Package{A1, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.UpgradeUniverse(true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(1))
			Expect(uninstall[0].GetName()).To(Equal("a"))
			Expect(uninstall[0].GetVersion()).To(Equal("1.1"))

			Expect(solution).To(ContainElement(PackageAssert{Package: A1, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: false}))

			Expect(solution).ToNot(ContainElement(PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(PackageAssert{Package: A, Value: true}))

			Expect(len(solution)).To(Equal(3))
		})

		It("Suggests to remove untracked packages", func() {
			for _, p := range []pkg.Package{E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []pkg.Package{A, B, C, E} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.UpgradeUniverse(true)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(uninstall)).To(Equal(3))

			Expect(uninstall).To(ContainElement(B))
			Expect(uninstall).To(ContainElement(A))
			Expect(uninstall).To(ContainElement(C))

			Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: false}))
			Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: false}))

			Expect(len(solution)).To(Equal(3))
		})
	})
})
