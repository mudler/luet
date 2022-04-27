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
	types "github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/solver"
)

var _ = Describe("Solver", func() {

	db := pkg.NewInMemoryDatabase(false)
	dbInstalled := pkg.NewInMemoryDatabase(false)
	dbDefinitions := pkg.NewInMemoryDatabase(false)
	s := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

	BeforeEach(func() {
		db = pkg.NewInMemoryDatabase(false)
		dbInstalled = pkg.NewInMemoryDatabase(false)
		dbDefinitions = pkg.NewInMemoryDatabase(false)
		s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)
	})

	Context("Select of best available package", func() {
		for i := 0; i < 200; i++ {

			It("picks the best versions available for each package, excluding the ones manually specified while installing", func() {

				B1 := types.NewPackage("B", "1.1", []*types.Package{}, []*types.Package{})
				B2 := types.NewPackage("B", "1.2", []*types.Package{}, []*types.Package{})
				B3 := types.NewPackage("B", "1.3", []*types.Package{}, []*types.Package{})
				B4 := types.NewPackage("B", "1.4", []*types.Package{}, []*types.Package{})

				A1 := types.NewPackage("A", "1.1", []*types.Package{
					types.NewPackage("B", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})
				A2 := types.NewPackage("A", "1.2", []*types.Package{
					types.NewPackage("B", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})

				D := types.NewPackage("D", "1.0", []*types.Package{
					types.NewPackage("A", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})

				C := types.NewPackage("C", "1", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A1, A2, B1, B2, B3, B4, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.(*Solver).Install([]*types.Package{D})

				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(8))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B4, Value: true}))

				Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B2, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B3, Value: false}))

				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err = s.(*Solver).Install([]*types.Package{D, B2})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(8))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B2, Value: true}))

				Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B4, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B3, Value: false}))

			})

			It("picks the best available excluding those manually input. In this case we the input is a selector >=0", func() {

				B1 := types.NewPackage("B", "1.1", []*types.Package{}, []*types.Package{})
				B2 := types.NewPackage("B", "1.2", []*types.Package{}, []*types.Package{})
				B3 := types.NewPackage("B", "1.3", []*types.Package{}, []*types.Package{})
				B4 := types.NewPackage("B", "1.4", []*types.Package{}, []*types.Package{})

				A1 := types.NewPackage("A", "1.1", []*types.Package{
					types.NewPackage("B", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})
				A2 := types.NewPackage("A", "1.2", []*types.Package{
					types.NewPackage("B", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})

				D := types.NewPackage("D", "1.0", []*types.Package{
					types.NewPackage("A", ">=0", []*types.Package{}, []*types.Package{}),
				}, []*types.Package{})

				C := types.NewPackage("C", "1", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A1, A2, B1, B2, B3, B4, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.(*Solver).Install([]*types.Package{types.NewPackage("D", ">=0", []*types.Package{}, []*types.Package{})})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(8))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B4, Value: true}))

				Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B2, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B3, Value: false}))

				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err = s.(*Solver).Install([]*types.Package{types.NewPackage("D", ">=0", []*types.Package{}, []*types.Package{}), B2})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(8))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B2, Value: true}))

				Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B1, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B4, Value: false}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B3, Value: false}))

			})

		}
	})

	Context("Simple set", func() {
		It("Solves correctly if the selected package has no requirements or conflicts and we have nothing installed yet", func() {

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("Solves correctly if the selected package has no requirements or conflicts and we have installed one package", func() {

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{B})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(2))
		})

		It("Solves correctly if the selected package to install has no requirement or conflicts, but in the world there is one with a requirement", func() {

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{B}, []*types.Package{})
			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{E, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: E, Value: true}))
			//	Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: false}))
			//Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))

			Expect(len(solution)).To(Equal(5))
		})

		It("Solves correctly if the selected package to install has requirements", func() {

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{D}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))

			Expect(len(solution)).To(Equal(3))
		})

		It("Solves correctly", func() {

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(err).ToNot(HaveOccurred())
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
		})
		It("Solves correctly more complex ones", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves correctly more complex ones", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves deps with expansion", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{D}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "B", Version: ">1.0"}}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Solves deps with more expansion", func() {

			C := types.NewPackage("c", "", []*types.Package{&types.Package{Name: "a", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			C.SetCategory("test")
			B := types.NewPackage("b", "1.0", []*types.Package{}, []*types.Package{})
			B.SetCategory("test")
			A := types.NewPackage("a", "1.1", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
			A.SetCategory("test")

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{C})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Solves deps with more expansion", func() {

			C := types.NewPackage("c", "1.5", []*types.Package{&types.Package{Name: "a", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			C.SetCategory("test")
			B := types.NewPackage("b", "1.0", []*types.Package{}, []*types.Package{})
			B.SetCategory("test")
			A := types.NewPackage("a", "1.1", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
			A.SetCategory("test")

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{&types.Package{Name: "c", Version: ">1.0", Category: "test"}})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Solves deps with more expansion", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "B", Version: ">=1.0"}}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(len(solution)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
		})
		It("Selects one version", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D2 := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "1.8", []*types.Package{}, []*types.Package{})
			D1 := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: "1.4"}}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, D1, D2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A, B})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D2, Value: false}))

			Expect(len(solution)).To(Equal(5))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Install only package requires", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			C := types.NewPackage("C", "1.1", []*types.Package{&types.Package{
				Name:    "A",
				Version: ">=1.0",
			}}, []*types.Package{})
			D := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{
				&types.Package{
					Name:    "D",
					Version: ">=0",
				},
			}, []*types.Package{})

			A := types.NewPackage("A", "1.2", []*types.Package{
				&types.Package{
					Name:    "D",
					Version: ">=1.0",
				},
			}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{C})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Selects best version", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			E.SetCategory("test")
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			C.SetCategory("test")
			D2 := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			D2.SetCategory("test")
			D := types.NewPackage("D", "1.8", []*types.Package{}, []*types.Package{})
			D.SetCategory("test")
			D1 := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			D1.SetCategory("test")
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			B.SetCategory("test")
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "D", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			A.SetCategory("test")

			for _, p := range []*types.Package{A, B, C, D, D1, D2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A, B})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D1, Value: false}))

			Expect(len(solution)).To(Equal(5))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides", func() {

			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			E.SetCategory("test")
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			C.SetCategory("test")
			D2 := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			D2.SetCategory("test")
			D := types.NewPackage("D", "1.8", []*types.Package{}, []*types.Package{})
			D.SetCategory("test")
			D1 := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			D1.SetCategory("test")
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			B.SetCategory("test")
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "D", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			A.SetCategory("test")

			D2.SetProvides([]*types.Package{{Name: "E", Category: "test"}})
			A2 := types.NewPackage("A", "1.3", []*types.Package{&types.Package{Name: "E", Version: "", Category: "test"}}, []*types.Package{})
			A2.SetCategory("test")

			for _, p := range []*types.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A2, B})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides with versions", func() {
			E := types.NewPackage("E", "1.3", []*types.Package{}, []*types.Package{})

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D2 := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "1.8", []*types.Package{}, []*types.Package{})
			D1 := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})

			D2.SetProvides([]*types.Package{{Name: "E", Version: "1.3"}})
			A2 := types.NewPackage("A", "1.3", []*types.Package{&types.Package{Name: "E", Version: ">=1.0"}}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A2})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Support provides with selectors", func() {
			E := types.NewPackage("E", "1.3", []*types.Package{}, []*types.Package{})

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D2 := types.NewPackage("D", "1.9", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "1.8", []*types.Package{}, []*types.Package{})
			D1 := types.NewPackage("D", "1.4", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "1.1", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{&types.Package{Name: "D", Version: ">=1.0"}}, []*types.Package{})

			D2.SetProvides([]*types.Package{{Name: "E", Version: ">=1.3"}})
			A2 := types.NewPackage("A", "1.3", []*types.Package{&types.Package{Name: "E", Version: ">=1.0"}}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, D1, D2, A2, E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

			solution, err := s.Install([]*types.Package{A2})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A2, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D1, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: D2, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D1, Value: false}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("Uninstall", func() {
			It("Uninstalls simple package correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("Uninstalls simple package expanded correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "1.2", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.Uninstall(true, true, &types.Package{Name: "A", Version: ">1.0"})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("Uninstalls simple packages not in world correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.Uninstall(true, true, A)
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})

			It("Uninstalls complex packages not in world correctly", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

				for _, p := range []*types.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
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
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				C := types.NewPackage("C", "", []*types.Package{B}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
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
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{C}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, C, D} {
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
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				//	C // installed

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
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
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
				C := types.NewPackage("C", "", []*types.Package{B}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				Z := types.NewPackage("Z", "", []*types.Package{A}, []*types.Package{})
				F := types.NewPackage("F", "", []*types.Package{Z, B}, []*types.Package{})

				Z.SetVersion("1.4101.dvw.dqc.")
				B.SetVersion("1.4101qe.eq.ff..dvw.dqc.")
				C.SetVersion("1.aaaa.eq.ff..dvw.dqc.")

				for _, p := range []*types.Package{A, B, C, D, Z, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D, Z, F} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Uninstall(true, false, B)
				Expect(err).To(HaveOccurred())
				Expect(len(solution)).To(Equal(0))
			})

			It("UninstallUniverse simple package correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("UninstallUniverse simple package expanded correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "1.2", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				s = NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, dbInstalled, dbDefinitions, db)

				solution, err := s.UninstallUniverse(types.Packages{
					&types.Package{Name: "A", Version: ">1.0"}})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})
			It("UninstallUniverse simple packages not in world correctly", func() {

				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))

				//	Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(len(solution)).To(Equal(1))
			})

			It("UninstallUniverse complex packages not in world correctly", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

				for _, p := range []*types.Package{B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))

				Expect(len(solution)).To(Equal(2))
			})

			It("UninstallUniverse complex packages correctly, even if shared deps are required by system packages", func() {
				// Here we diff a lot from standard Uninstall:
				// all the packages that has reverse deps will be removed (aka --full)
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				C := types.NewPackage("C", "", []*types.Package{B}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}
				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))
				Expect(solution).To(ContainElement(C))

				Expect(len(solution)).To(Equal(3))
			})

			It("UninstallUniverse complex packages in world correctly", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{C}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(C))

				Expect(len(solution)).To(Equal(2))
			})

			It("UninstallUniverse complex package correctly", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				//	C // installed

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.UninstallUniverse(types.Packages{A})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).To(ContainElement(A))
				Expect(solution).To(ContainElement(B))
				Expect(solution).To(ContainElement(D))

				Expect(len(solution)).To(Equal(3))

			})

		})
		It("Find conflicts", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(A)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())

		})

		It("Find nested conflicts", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{D}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(D)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(BeTrue())
		})

		It("Doesn't find nested conflicts", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{D}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Doesn't find conflicts", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			val, err := s.ConflictsWithInstalled(C)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Find conflicts using revdeps", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(A, dbInstalled.World())
			Expect(err.Error()).To(Equal("\nB"))
			Expect(val).To(BeTrue())

		})

		It("Find nested conflicts with revdeps", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{D}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(D, dbInstalled.World())
			Expect(err.Error()).To(Or(Equal("\nA\nB"), Equal("\nB\nA")))
			Expect(val).To(BeTrue())
		})

		It("Doesn't find nested conflicts with revdeps", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{D}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{A}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			val, err := s.Conflicts(C, dbInstalled.World())
			Expect(err).ToNot(HaveOccurred())
			Expect(val).ToNot(BeTrue())
		})

		It("Doesn't find conflicts with revdeps", func() {

			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{}, []*types.Package{})

			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, D} {
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
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			//	D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
			B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{C})
			A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{A})
			Expect(len(solution)).To(Equal(0))
			Expect(err).To(HaveOccurred())
		})

	})

	Context("Complex data sets", func() {
		It("Solves them correctly", func() {
			C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
			E := types.NewPackage("E", "", []*types.Package{}, []*types.Package{})
			F := types.NewPackage("F", "", []*types.Package{}, []*types.Package{})
			G := types.NewPackage("G", "", []*types.Package{}, []*types.Package{})
			H := types.NewPackage("H", "", []*types.Package{G}, []*types.Package{})
			D := types.NewPackage("D", "", []*types.Package{H}, []*types.Package{})
			B := types.NewPackage("B", "", []*types.Package{D}, []*types.Package{})
			A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})

			for _, p := range []*types.Package{A, B, C, D, E, F, G} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			solution, err := s.Install([]*types.Package{A})
			Expect(err).ToNot(HaveOccurred())

			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: H, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: G, Value: true}))

			Expect(len(solution)).To(Equal(6))
		})
	})

	Context("Selection", func() {
		a := types.NewPackage("A", ">=2.0", []*types.Package{}, []*types.Package{})
		a1 := types.NewPackage("A", "2.0", []*types.Package{}, []*types.Package{})
		a11 := types.NewPackage("A", "2.1", []*types.Package{}, []*types.Package{})
		a01 := types.NewPackage("A", "2.2", []*types.Package{}, []*types.Package{})
		a02 := types.NewPackage("A", "2.3", []*types.Package{}, []*types.Package{})
		a03 := types.NewPackage("A", "2.3.4", []*types.Package{}, []*types.Package{})
		old := types.NewPackage("A", "1.3.1", []*types.Package{}, []*types.Package{})

		It("Expands correctly", func() {
			definitions := pkg.NewInMemoryDatabase(false)
			for _, p := range []*types.Package{a1, a11, a01, a02, a03, old} {
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
		E := types.NewPackage("e", "1.5", []*types.Package{}, []*types.Package{})
		E.SetCategory("test")
		C := types.NewPackage("c", "1.5", []*types.Package{&types.Package{Name: "a", Version: ">=1.0", Category: "test"}}, []*types.Package{})
		C.SetCategory("test")
		B := types.NewPackage("b", "1.0", []*types.Package{}, []*types.Package{})
		B.SetCategory("test")
		A := types.NewPackage("a", "1.1", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
		A.SetCategory("test")
		A1 := types.NewPackage("a", "1.2", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
		A1.SetCategory("test")

		BeforeEach(func() {
			C = types.NewPackage("c", "1.5", []*types.Package{&types.Package{Name: "a", Version: ">=1.0", Category: "test"}}, []*types.Package{})
			C.SetCategory("test")
			B = types.NewPackage("b", "1.0", []*types.Package{}, []*types.Package{})
			B.SetCategory("test")
			A = types.NewPackage("a", "1.1", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
			A.SetCategory("test")
			A1 = types.NewPackage("a", "1.2", []*types.Package{&types.Package{Name: "b", Version: "1.0", Category: "test"}}, []*types.Package{})
			A1.SetCategory("test")
		})

		It("upgrades correctly", func() {
			for _, p := range []*types.Package{A1, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(1))
			Expect(uninstall[0].GetName()).To(Equal("a"))
			Expect(uninstall[0].GetVersion()).To(Equal("1.1"))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: false}))
			Expect(len(solution)).To(Equal(3))
		})

		It("upgrades correctly with provides", func() {
			B.SetProvides([]*types.Package{
				&types.Package{Name: "a", Version: ">=0", Category: "test"},
				&types.Package{Name: "c", Version: ">=0", Category: "test"},
			})

			for _, p := range []*types.Package{B} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(2))

			Expect(uninstall).To(ContainElement(C))
			Expect(uninstall).To(ContainElement(A))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		PIt("upgrades correctly with provides, also if definitiondb contains both a provide, and the package to be provided", func() {
			B.SetProvides([]*types.Package{
				&types.Package{Name: "a", Version: ">=0", Category: "test"},
				&types.Package{Name: "c", Version: ">=0", Category: "test"},
			})

			for _, p := range []*types.Package{A1, B} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, C} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.Upgrade(true, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(2))

			Expect(uninstall).To(ContainElement(C))
			Expect(uninstall).To(ContainElement(A))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(len(solution)).To(Equal(1))
		})

		It("UpgradeUniverse upgrades correctly", func() {
			for _, p := range []*types.Package{A1, B, C} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.UpgradeUniverse(true)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uninstall)).To(Equal(1))
			Expect(uninstall[0].GetName()).To(Equal("a"))
			Expect(uninstall[0].GetVersion()).To(Equal("1.1"))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: A1, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: false}))

			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))

			Expect(len(solution)).To(Equal(3))
		})

		It("Suggests to remove untracked packages", func() {
			for _, p := range []*types.Package{E} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{A, B, C, E} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			uninstall, solution, err := s.UpgradeUniverse(true)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(uninstall)).To(Equal(3))

			Expect(uninstall).To(ContainElement(B))
			Expect(uninstall).To(ContainElement(A))
			Expect(uninstall).To(ContainElement(C))

			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: false}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: false}))

			Expect(len(solution)).To(Equal(3))
		})
	})
})
