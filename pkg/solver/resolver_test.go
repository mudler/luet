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

var _ = Describe("Resolver", func() {

	db := pkg.NewInMemoryDatabase(false)
	dbInstalled := pkg.NewInMemoryDatabase(false)
	dbDefinitions := pkg.NewInMemoryDatabase(false)
	s := NewSolver(dbInstalled, dbDefinitions, db)

	BeforeEach(func() {
		db = pkg.NewInMemoryDatabase(false)
		dbInstalled = pkg.NewInMemoryDatabase(false)
		dbDefinitions = pkg.NewInMemoryDatabase(false)
		s = NewSolver(dbInstalled, dbDefinitions, db)
	})

	Context("Conflict set", func() {
		Context("DummyPackageResolver", func() {
			It("is unsolvable - as we something we ask to install conflict with system stuff", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
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
			It("succeeds to install D and F if explictly requested", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				F := pkg.NewPackage("F", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D, E, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]pkg.Package{D, F}) // D and F should go as they have no deps. A/E should be filtered by QLearn
				Expect(err).ToNot(HaveOccurred())

				Expect(len(solution)).To(Equal(6))

				Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(PackageAssert{Package: E, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: F, Value: true}))

			})

		})
		Context("QLearningResolver", func() {
			It("will find out that we can install D by ignoring A", func() {
				s.SetResolver(&QLearningResolver{})
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]pkg.Package{A, D})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(solution)).To(Equal(4))

				Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
			})

			It("will find out that we can install D and F by ignoring E and A", func() {
				s.SetResolver(&QLearningResolver{})
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				E := pkg.NewPackage("E", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				F := pkg.NewPackage("F", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D, E, F} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]pkg.Package{A, D, E, F}) // D and F should go as they have no deps. A/E should be filtered by QLearn
				Expect(err).ToNot(HaveOccurred())

				Expect(len(solution)).To(Equal(6))

				Expect(solution).To(ContainElement(PackageAssert{Package: A, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: B, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: C, Value: true})) // Was already installed
				Expect(solution).To(ContainElement(PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(PackageAssert{Package: E, Value: false}))
				Expect(solution).To(ContainElement(PackageAssert{Package: F, Value: true}))
			})
		})

		Context("DummyPackageResolver", func() {
			It("cannot find a solution", func() {
				C := pkg.NewPackage("C", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
				B := pkg.NewPackage("B", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{C})
				A := pkg.NewPackage("A", "", []*pkg.DefaultPackage{B}, []*pkg.DefaultPackage{})
				D := pkg.NewPackage("D", "", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})

				for _, p := range []pkg.Package{A, B, C, D} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []pkg.Package{C} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]pkg.Package{A, D})
				Expect(err).To(HaveOccurred())

				Expect(len(solution)).To(Equal(0))
			})

		})
	})

})
