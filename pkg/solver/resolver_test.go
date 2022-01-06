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
	"github.com/mudler/luet/pkg/api/core/types"

	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/solver"
)

var _ = Describe("Resolver", func() {

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

	Context("Conflict set", func() {
		Context("Explainer", func() {
			It("is unsolvable - as we something we ask to install conflict with system stuff", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
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
			It("succeeds to install D and F if explictly requested", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{C})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				E := types.NewPackage("E", "", []*types.Package{B}, []*types.Package{})
				F := types.NewPackage("F", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D, E, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]*types.Package{D, F}) // D and F should go as they have no deps. A/E should be filtered by QLearn
				Expect(err).ToNot(HaveOccurred())

				Expect(len(solution)).To(Equal(6))

				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))
				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: F, Value: true}))

			})

		})
		Context("QLearningResolver", func() {
			It("will find out that we can install D by ignoring A", func() {
				s.SetResolver(SimpleQLearningSolver())
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{C})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]*types.Package{A, D})
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))
				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))

				Expect(len(solution)).To(Equal(4))
			})

			It("will find out that we can install D and F by ignoring E and A", func() {
				s.SetResolver(SimpleQLearningSolver())
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{C})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})
				E := types.NewPackage("E", "", []*types.Package{B}, []*types.Package{})
				F := types.NewPackage("F", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D, E, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]*types.Package{A, D, E, F}) // D and F should go as they have no deps. A/E should be filtered by QLearn
				Expect(err).ToNot(HaveOccurred())

				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: A, Value: true}))
				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true})) // Was already installed
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).ToNot(ContainElement(types.PackageAssert{Package: E, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: F, Value: true}))
				Expect(len(solution)).To(Equal(6))
			})
		})

		Context("Explainer", func() {
			It("cannot find a solution", func() {
				C := types.NewPackage("C", "", []*types.Package{}, []*types.Package{})
				B := types.NewPackage("B", "", []*types.Package{}, []*types.Package{C})
				A := types.NewPackage("A", "", []*types.Package{B}, []*types.Package{})
				D := types.NewPackage("D", "", []*types.Package{}, []*types.Package{})

				for _, p := range []*types.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]*types.Package{A, D})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`could not satisfy the constraints: 
A-- and 
C-- and 
!(A--) or B-- and 
!(B--) or !(C--)`))

				Expect(len(solution)).To(Equal(0))
			})

		})
	})

})
