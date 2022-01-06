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
	"strconv"

	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/solver"
)

var _ = Describe("Decoder", func() {
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

	Context("Assertion ordering", func() {
		eq := 0
		for index := 0; index < 300; index++ { // Just to make sure we don't have false positives
			It("Orders them correctly #"+strconv.Itoa(index), func() {

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
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: H, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: G, Value: true}))

				Expect(len(solution)).To(Equal(6))
				Expect(err).ToNot(HaveOccurred())
				solution, err = solution.Order(dbDefinitions, A.GetFingerPrint())
				Expect(err).ToNot(HaveOccurred())
				//	Expect(len(solution)).To(Equal(6))
				Expect(solution[0].Package.GetName()).To(Equal("G"))
				Expect(solution[1].Package.GetName()).To(Equal("H"))
				Expect(solution[2].Package.GetName()).To(Equal("D"))
				Expect(solution[3].Package.GetName()).To(Equal("B"))
				eq += len(solution)
				//Expect(solution[4].Package.GetName()).To(Equal("A"))
				//Expect(solution[5].Package.GetName()).To(Equal("C")) As C doesn't have any dep it can be in both positions
			})
		}

		It("Expects perfect equality when ordered", func() {
			Expect(eq).To(Equal(300 * 5)) // assertions lenghts
		})

		disequality := 0
		equality := 0
		for index := 0; index < 300; index++ { // Just to make sure we don't have false positives
			It("Doesn't order them correctly otherwise #"+strconv.Itoa(index), func() {

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
				Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: H, Value: true}))
				Expect(solution).To(ContainElement(types.PackageAssert{Package: G, Value: true}))

				Expect(len(solution)).To(Equal(6))
				Expect(err).ToNot(HaveOccurred())
				if solution[0].Package.GetName() != "G" {
					disequality++
				} else {
					equality++
				}
				if solution[1].Package.GetName() != "H" {
					disequality++
				} else {
					equality++
				}
				if solution[2].Package.GetName() != "D" {
					disequality++
				} else {
					equality++
				}
				if solution[3].Package.GetName() != "B" {
					disequality++
				} else {
					equality++
				}
				if solution[4].Package.GetName() != "A" {
					disequality++
				} else {
					equality++
				}

			})
			It("Expect disequality", func() {
				Expect(disequality).ToNot(Equal(0))
				Expect(equality).ToNot(Equal(300 * 6))
			})
		}
	})

	Context("Assertion hashing", func() {
		It("Hashes them, and could be used for comparison", func() {

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
			Expect(solution).To(ContainElement(types.PackageAssert{Package: A, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: H, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: G, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
			solution, err = solution.Order(dbDefinitions, A.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			//	Expect(len(solution)).To(Equal(6))
			Expect(solution[0].Package.GetName()).To(Equal("G"))
			Expect(solution[1].Package.GetName()).To(Equal("H"))
			Expect(solution[2].Package.GetName()).To(Equal("D"))
			Expect(solution[3].Package.GetName()).To(Equal("B"))

			hash := solution.AssertionHash()

			solution, err = s.Install([]*types.Package{B})
			Expect(solution).To(ContainElement(types.PackageAssert{Package: B, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: D, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: C, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: H, Value: true}))
			Expect(solution).To(ContainElement(types.PackageAssert{Package: G, Value: true}))

			Expect(len(solution)).To(Equal(6))
			Expect(err).ToNot(HaveOccurred())
			solution, err = solution.Order(dbDefinitions, B.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			hash2 := solution.AssertionHash()

			//	Expect(len(solution)).To(Equal(6))
			Expect(solution[0].Package.GetName()).To(Equal("G"))
			Expect(solution[1].Package.GetName()).To(Equal("H"))
			Expect(solution[2].Package.GetName()).To(Equal("D"))
			Expect(solution[3].Package.GetName()).To(Equal("B"))

			Expect(hash).ToNot(Equal(""))
			Expect(hash2).ToNot(Equal(""))
			Expect(hash != hash2).To(BeTrue())

		})
		It("Hashes them, and could be used for comparison", func() {

			X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			Y := types.NewPackage("Y", "", []*types.Package{X}, []*types.Package{})
			Z := types.NewPackage("Z", "", []*types.Package{X}, []*types.Package{})

			for _, p := range []*types.Package{X, Y, Z} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{Y})
			Expect(err).ToNot(HaveOccurred())

			solution2, err := s.Install([]*types.Package{Z})
			Expect(err).ToNot(HaveOccurred())
			orderY, err := solution.Order(dbDefinitions, Y.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			orderZ, err := solution2.Order(dbDefinitions, Z.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			Expect(orderY.Drop(Y).AssertionHash() == orderZ.Drop(Z).AssertionHash()).To(BeTrue())
		})

		It("Hashes them, Cuts them and could be used for comparison", func() {

			X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			Y := types.NewPackage("Y", "", []*types.Package{X}, []*types.Package{})
			Z := types.NewPackage("Z", "", []*types.Package{X}, []*types.Package{})

			for _, p := range []*types.Package{X, Y, Z} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{Y})
			Expect(err).ToNot(HaveOccurred())

			solution2, err := s.Install([]*types.Package{Z})
			Expect(err).ToNot(HaveOccurred())
			orderY, err := solution.Order(dbDefinitions, Y.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			orderZ, err := solution2.Order(dbDefinitions, Z.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			Expect(orderY.Cut(Y).Drop(Y)).To(Equal(orderZ.Cut(Z).Drop(Z)))

			Expect(orderY.Cut(Y).Drop(Y).AssertionHash()).To(Equal(orderZ.Cut(Z).Drop(Z).AssertionHash()))
		})

		It("HashFrom can be used equally", func() {

			X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			Y := types.NewPackage("Y", "", []*types.Package{X}, []*types.Package{})
			Z := types.NewPackage("Z", "", []*types.Package{X}, []*types.Package{})

			for _, p := range []*types.Package{X, Y, Z} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{Y})
			Expect(err).ToNot(HaveOccurred())

			solution2, err := s.Install([]*types.Package{Z})
			Expect(err).ToNot(HaveOccurred())
			orderY, err := solution.Order(dbDefinitions, Y.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			orderZ, err := solution2.Order(dbDefinitions, Z.GetFingerPrint())
			Expect(err).ToNot(HaveOccurred())
			Expect(orderY.Cut(Y).Drop(Y)).To(Equal(orderZ.Cut(Z).Drop(Z)))

			Expect(orderY.Cut(Y).HashFrom(Y)).To(Equal(orderZ.Cut(Z).HashFrom(Z)))
		})

		It("Unique hashes for single packages", func() {

			X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			F := types.NewPackage("F", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})

			for _, p := range []*types.Package{X, F, D} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{X})
			Expect(err).ToNot(HaveOccurred())

			solution2, err := s.Install([]*types.Package{F})
			Expect(err).ToNot(HaveOccurred())

			solution3, err := s.Install([]*types.Package{D})
			Expect(err).ToNot(HaveOccurred())

			Expect(solution.AssertionHash()).ToNot(Equal(solution2.AssertionHash()))
			Expect(solution3.AssertionHash()).To(Equal(solution.AssertionHash()))

		})

		It("Unique hashes for empty assertions", func() {
			empty := types.PackagesAssertions{}
			empty2 := types.PackagesAssertions{}

			Expect(empty.AssertionHash()).To(Equal(empty2.AssertionHash()))
		})

		It("Unique hashes for single packages with HashFrom", func() {

			X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			F := types.NewPackage("F", "", []*types.Package{}, []*types.Package{})
			D := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
			Y := types.NewPackage("Y", "", []*types.Package{X}, []*types.Package{})

			empty := types.PackagesAssertions{}

			for _, p := range []*types.Package{X, F, D, Y} {
				_, err := dbDefinitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, p := range []*types.Package{} {
				_, err := dbInstalled.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			solution, err := s.Install([]*types.Package{X})
			Expect(err).ToNot(HaveOccurred())

			solution2, err := s.Install([]*types.Package{F})
			Expect(err).ToNot(HaveOccurred())

			solution3, err := s.Install([]*types.Package{D})
			Expect(err).ToNot(HaveOccurred())

			solution4, err := s.Install([]*types.Package{Y})
			Expect(err).ToNot(HaveOccurred())

			Expect(solution.HashFrom(X)).ToNot(Equal(solution2.HashFrom(F)))
			Expect(solution3.HashFrom(D)).To(Equal(solution.HashFrom(X)))
			Expect(solution3.SaltedHashFrom(D, map[string]string{D.GetFingerPrint(): "foo"})).ToNot(Equal(solution3.HashFrom(D)))

			Expect(solution4.SaltedHashFrom(Y, map[string]string{X.GetFingerPrint(): "foo"})).ToNot(Equal(solution4.HashFrom(Y)))

			Expect(empty.AssertionHash()).ToNot(Equal(solution3.HashFrom(D)))
			Expect(empty.AssertionHash()).ToNot(Equal(solution2.HashFrom(F)))

			Expect(solution4.Drop(Y).AssertionHash()).To(Equal(solution4.HashFrom(Y)))
		})
		for index := 0; index < 300; index++ { // Just to make sure we don't have false positives

			It("Always same solution", func() {

				X := types.NewPackage("X", "", []*types.Package{}, []*types.Package{})
				Y := types.NewPackage("Y", "", []*types.Package{X}, []*types.Package{})
				Z := types.NewPackage("Z", "", []*types.Package{X}, []*types.Package{})
				W := types.NewPackage("W", "", []*types.Package{Z, Y}, []*types.Package{})

				for _, p := range []*types.Package{X, Y, Z} {
					_, err := dbDefinitions.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				for _, p := range []*types.Package{} {
					_, err := dbInstalled.CreatePackage(p)
					Expect(err).ToNot(HaveOccurred())
				}

				solution, err := s.Install([]*types.Package{W})
				Expect(err).ToNot(HaveOccurred())

				orderW, err := solution.Order(dbDefinitions, W.GetFingerPrint())
				Expect(err).ToNot(HaveOccurred())
				Expect(len(orderW) > 0).To(BeTrue())
				Expect(orderW[0].Package.GetName()).To(Equal("X"))
				Expect(orderW[1].Package.GetName()).To(Equal("Y"))
				Expect(orderW[2].Package.GetName()).To(Equal("Z"))
				Expect(orderW[3].Package.GetName()).To(Equal("W"))
			})

		}

	})
})
