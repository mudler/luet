// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or ItNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package pkg_test

import (
	"io/ioutil"
	"os"
	"regexp"

	. "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BoltDB Database", func() {

	tmpfile, _ := ioutil.TempFile(os.TempDir(), "tests")
	defer os.Remove(tmpfile.Name()) // clean up
	var db PackageDatabase

	BeforeEach(func() {
		tmpfile, _ = ioutil.TempFile(os.TempDir(), "tests")
		defer os.Remove(tmpfile.Name()) // clean up
		db = NewBoltDatabase(tmpfile.Name())
	})
	Context("Simple package", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

		It("Find packages", func() {
			ID, err := db.CreatePackage(a)
			Expect(err).ToNot(HaveOccurred())

			pack, err := db.GetPackage(ID)
			Expect(err).ToNot(HaveOccurred())

			Expect(pack).To(Equal(a))
			ids := db.GetPackages()

			Expect(ids).To(Equal([]string{"1"}))

			pack, err = db.FindPackage(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(pack).To(Equal(a))

		})

		It("Find package files", func() {
			a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})
			_, err := db.CreatePackage(a)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a1)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a3)
			Expect(err).ToNot(HaveOccurred())

			err = db.SetPackageFiles(&PackageFile{PackageFingerprint: a.GetFingerPrint(), Files: []string{"foo"}})
			Expect(err).ToNot(HaveOccurred())

			err = db.SetPackageFiles(&PackageFile{PackageFingerprint: a1.GetFingerPrint(), Files: []string{"bar"}})
			Expect(err).ToNot(HaveOccurred())

			pack, err := db.FindPackageByFile("fo")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pack)).To(Equal(1))
			Expect(pack[0]).To(Equal(a))
		})

		It("Expands correctly", func() {

			a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a01 := NewPackage("A", "0.1", []*DefaultPackage{}, []*DefaultPackage{})
			re := regexp.MustCompile("project[0-9][=].*")
			for _, p := range []Package{a1, a11, a01} {
				_, err := db.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst, err := a.Expand(db)
			Expect(err).ToNot(HaveOccurred())
			Expect(lst).To(ContainElement(a11))
			Expect(lst).To(ContainElement(a1))
			Expect(lst).ToNot(ContainElement(a01))
			Expect(len(lst)).To(Equal(2))
			p := lst.Best(nil)
			Expect(p).To(Equal(a11))
			// Test annotation with null map
			Expect(a.MatchAnnotation(re)).To(Equal(false))
		})

		It("Find best package candidate", func() {
			a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})
			_, err := db.CreatePackage(a)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a1)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a3)
			Expect(err).ToNot(HaveOccurred())
			s := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

			pack, err := db.FindPackageCandidate(s)
			Expect(err).ToNot(HaveOccurred())
			Expect(pack).To(Equal(a3))

		})

		It("Find specific package candidate", func() {
			a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})
			_, err := db.CreatePackage(a)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a1)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a3)
			Expect(err).ToNot(HaveOccurred())
			s := NewPackage("A", "=1.0", []*DefaultPackage{}, []*DefaultPackage{})

			pack, err := db.FindPackageCandidate(s)
			Expect(err).ToNot(HaveOccurred())
			Expect(pack).To(Equal(a))

		})

		It("Provides replaces definitions", func() {
			a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

			a3.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}})
			Expect(a3.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}}))

			_, err := db.CreatePackage(a)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a1)
			Expect(err).ToNot(HaveOccurred())

			_, err = db.CreatePackage(a3)
			Expect(err).ToNot(HaveOccurred())

			s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

			pack, err := db.FindPackage(s)
			Expect(err).ToNot(HaveOccurred())
			Expect(pack).To(Equal(a3))
		})

		Context("Provides", func() {

			It("replaces definitions", func() {
				a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
				a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
				a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

				a3.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}})
				Expect(a3.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}}))

				_, err := db.CreatePackage(a)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(a1)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(a3)
				Expect(err).ToNot(HaveOccurred())

				s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

				pack, err := db.FindPackage(s)
				Expect(err).ToNot(HaveOccurred())
				Expect(pack).To(Equal(a3))
			})

			It("replaces definitions", func() {
				a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
				a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
				a3 := NewPackage("A", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

				a3.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}})
				Expect(a3.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: "1.0"}}))

				_, err := db.CreatePackage(a)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(a1)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(a3)
				Expect(err).ToNot(HaveOccurred())

				s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

				packs, err := db.FindPackages(s)
				Expect(err).ToNot(HaveOccurred())
				Expect(packs).To(ContainElement(a3))
			})

			It("replaces definitions", func() {
				a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
				a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
				z := NewPackage("Z", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

				z.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}})
				Expect(z.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}}))

				_, err := db.CreatePackage(a)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(a1)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(z)
				Expect(err).ToNot(HaveOccurred())

				s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

				packs, err := db.FindPackages(s)
				Expect(err).ToNot(HaveOccurred())
				Expect(packs).To(ContainElement(z))
			})

			It("replaces definitions of unexisting packages", func() {
				a1 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
				z := NewPackage("Z", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

				z.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}})
				Expect(z.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}}))

				_, err := db.CreatePackage(a1)
				Expect(err).ToNot(HaveOccurred())

				_, err = db.CreatePackage(z)
				Expect(err).ToNot(HaveOccurred())

				s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

				packs, err := db.FindPackages(s)
				Expect(err).ToNot(HaveOccurred())
				Expect(packs).To(ContainElement(z))
			})

			It("replaces definitions of a required package", func() {

				c := NewPackage("C", "1.1", []*DefaultPackage{{Name: "A", Category: "", Version: ">=0"}}, []*DefaultPackage{})
				z := NewPackage("Z", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

				z.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}})
				Expect(z.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}}))

				_, err := db.CreatePackage(z)
				Expect(err).ToNot(HaveOccurred())
				_, err = db.CreatePackage(c)
				Expect(err).ToNot(HaveOccurred())

				s := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

				packs, err := db.FindPackages(s)
				Expect(err).ToNot(HaveOccurred())
				Expect(packs).To(ContainElement(z))
			})

			When("Searching with selectors", func() {
				It("replaces definitions of a required package", func() {

					c := NewPackage("C", "1.1", []*DefaultPackage{{Name: "A", Category: "", Version: ">=0"}}, []*DefaultPackage{})
					z := NewPackage("Z", "1.3", []*DefaultPackage{}, []*DefaultPackage{})

					z.SetProvides([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}})
					Expect(z.GetProvides()).To(Equal([]*DefaultPackage{{Name: "A", Category: "", Version: ">=1.0"}}))

					_, err := db.CreatePackage(z)
					Expect(err).ToNot(HaveOccurred())
					_, err = db.CreatePackage(c)
					Expect(err).ToNot(HaveOccurred())

					s := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

					packs, err := db.FindPackages(s)
					Expect(err).ToNot(HaveOccurred())
					Expect(packs).To(ContainElement(z))
				})
			})

		})

	})

})
