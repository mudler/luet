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

package pkg_test

import (
	"regexp"

	. "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Package", func() {

	Context("Encoding/Decoding", func() {
		a := &DefaultPackage{Name: "test", Version: "1", Category: "t"}
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

		It("Encodes and decodes correctly", func() {
			Expect(a.String()).ToNot(Equal(""))
			p := FromString(a.String())
			Expect(p).To(Equal(a))
		})

		It("Generates packages fingerprint's hashes", func() {
			Expect(a.HashFingerprint()).ToNot(Equal(a1.HashFingerprint()))
			Expect(a.HashFingerprint()).To(Equal("c64caa391b79adb598ad98e261aa79a0"))
		})
	})

	Context("Simple package", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
		a01 := NewPackage("A", "0.1", []*DefaultPackage{}, []*DefaultPackage{})
		It("Expands correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a1, a11, a01} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst, err := a.Expand(definitions)
			Expect(err).ToNot(HaveOccurred())
			Expect(lst).To(ContainElement(a11))
			Expect(lst).To(ContainElement(a1))
			Expect(lst).ToNot(ContainElement(a01))
			Expect(len(lst)).To(Equal(2))
			p := lst.Best(nil)
			Expect(p).To(Equal(a11))
		})
	})

	Context("Find label on packages", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a.AddLabel("project1", "test1")
		a.AddLabel("label2", "value1")
		b := NewPackage("B", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b.AddLabel("project2", "test2")
		b.AddLabel("label2", "value1")
		It("Expands correctly", func() {
			var err error
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b} {
				_, err = definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			re := regexp.MustCompile("project[0-9][=].*")
			Expect(err).ToNot(HaveOccurred())
			Expect(re).ToNot(BeNil())
			Expect(a.HasLabel("label2")).To(Equal(true))
			Expect(a.HasLabel("label3")).To(Equal(false))
			Expect(a.HasLabel("project1")).To(Equal(true))
			Expect(b.HasLabel("project2")).To(Equal(true))
			Expect(b.HasLabel("label2")).To(Equal(true))
			Expect(b.MatchLabel(re)).To(Equal(true))
			Expect(a.MatchLabel(re)).To(Equal(true))

		})
	})

	Context("Check description", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a.SetDescription("Description A")

		It("Set and get correctly a description", func() {
			Expect(a.GetDescription()).To(Equal("Description A"))
		})
	})

	Context("Check licenses", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a.SetLicense("MIT")

		It("Set and get correctly a license", func() {
			Expect(a.GetLicense()).To(Equal("MIT"))
		})
	})

	Context("Check URI", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a.AddURI("ftp://ftp.freeradius.org/pub/radius/freearadius-server-3.0.20.tar.gz")

		It("Set and get correctly an uri", func() {
			Expect(a.GetURI()).To(Equal([]string{
				"ftp://ftp.freeradius.org/pub/radius/freearadius-server-3.0.20.tar.gz",
			}))
		})
	})

	Context("revdeps", func() {
		a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b := NewPackage("B", "1.0", []*DefaultPackage{a}, []*DefaultPackage{})
		c := NewPackage("C", "1.1", []*DefaultPackage{b}, []*DefaultPackage{})
		d := NewPackage("D", "0.1", []*DefaultPackage{}, []*DefaultPackage{})
		It("Computes correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b, c, d} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst := a.Revdeps(definitions)
			Expect(lst).To(ContainElement(b))
			Expect(lst).To(ContainElement(c))
			Expect(len(lst)).To(Equal(2))
		})
	})

	Context("revdeps", func() {
		a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b := NewPackage("B", "1.0", []*DefaultPackage{a}, []*DefaultPackage{})
		c := NewPackage("C", "1.1", []*DefaultPackage{b}, []*DefaultPackage{})
		d := NewPackage("D", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})
		e := NewPackage("E", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})

		It("Computes correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b, c, d, e} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst := a.Revdeps(definitions)
			Expect(lst).To(ContainElement(c))
			Expect(lst).To(ContainElement(d))
			Expect(lst).To(ContainElement(e))
			Expect(len(lst)).To(Equal(4))
		})
	})

	Context("revdeps", func() {
		a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b := NewPackage("B", "1.0", []*DefaultPackage{&DefaultPackage{Name: "A", Version: ">=1.0"}}, []*DefaultPackage{})
		c := NewPackage("C", "1.1", []*DefaultPackage{&DefaultPackage{Name: "B", Version: ">=1.0"}}, []*DefaultPackage{})
		d := NewPackage("D", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})
		e := NewPackage("E", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})

		It("doesn't resolve selectors", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b, c, d, e} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst := a.Revdeps(definitions)
			Expect(len(lst)).To(Equal(0))
		})
	})
	Context("Expandedrevdeps", func() {
		a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b := NewPackage("B", "1.0", []*DefaultPackage{&DefaultPackage{Name: "A", Version: ">=1.0"}}, []*DefaultPackage{})
		c := NewPackage("C", "1.1", []*DefaultPackage{&DefaultPackage{Name: "B", Version: ">=1.0"}}, []*DefaultPackage{})
		d := NewPackage("D", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})
		e := NewPackage("E", "0.1", []*DefaultPackage{c}, []*DefaultPackage{})

		It("Computes correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b, c, d, e} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst := a.ExpandedRevdeps(definitions)
			Expect(lst).To(ContainElement(c))
			Expect(lst).To(ContainElement(d))
			Expect(lst).To(ContainElement(e))
			Expect(len(lst)).To(Equal(4))
		})
	})

	Context("Expandedrevdeps", func() {
		a := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		b := NewPackage("B", "1.0", []*DefaultPackage{&DefaultPackage{Name: "A", Version: ">=1.0"}}, []*DefaultPackage{})
		c := NewPackage("C", "1.1", []*DefaultPackage{&DefaultPackage{Name: "B", Version: ">=1.0"}}, []*DefaultPackage{})
		d := NewPackage("D", "0.1", []*DefaultPackage{&DefaultPackage{Name: "C", Version: ">=1.0"}}, []*DefaultPackage{})
		e := NewPackage("E", "0.1", []*DefaultPackage{&DefaultPackage{Name: "C", Version: ">=1.0"}}, []*DefaultPackage{})

		It("Computes correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, b, c, d, e} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			lst := a.ExpandedRevdeps(definitions)
			Expect(lst).To(ContainElement(b))
			Expect(lst).To(ContainElement(c))
			Expect(lst).To(ContainElement(d))
			Expect(lst).To(ContainElement(e))
			Expect(len(lst)).To(Equal(4))
		})
	})

	Context("RequiresContains", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a1 := NewPackage("A", "1.0", []*DefaultPackage{a}, []*DefaultPackage{})
		a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
		a01 := NewPackage("A", "0.1", []*DefaultPackage{a1, a11}, []*DefaultPackage{})

		It("returns correctly", func() {
			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a, a1, a11, a01} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(a01.RequiresContains(definitions, a1)).To(BeTrue())
			Expect(a01.RequiresContains(definitions, a11)).To(BeTrue())
			Expect(a01.RequiresContains(definitions, a)).To(BeTrue())
			Expect(a.RequiresContains(definitions, a11)).ToNot(BeTrue())
			Expect(a.IsSelector()).To(BeTrue())
			Expect(a1.IsSelector()).To(BeFalse())

		})
	})

	Context("Encoding", func() {
		db := NewInMemoryDatabase(false)
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
		a := NewPackage("A", ">=1.0", []*DefaultPackage{a1}, []*DefaultPackage{a11})
		It("decodes and encodes correctly", func() {

			ID, err := a.Encode(db)
			Expect(err).ToNot(HaveOccurred())

			p, err := DecodePackage(ID, db)
			Expect(err).ToNot(HaveOccurred())

			Expect(p.GetVersion()).To(Equal(a.GetVersion()))
			Expect(p.GetName()).To(Equal(a.GetName()))
			Expect(p.Flagged()).To(Equal(a.Flagged()))
			Expect(p.GetFingerPrint()).To(Equal(a.GetFingerPrint()))
			Expect(len(p.GetConflicts())).To(Equal(len(a.GetConflicts())))
			Expect(len(p.GetRequires())).To(Equal(len(a.GetRequires())))
			Expect(len(p.GetRequires())).To(Equal(1))
			Expect(len(p.GetConflicts())).To(Equal(1))
			Expect(p.GetConflicts()[0].GetName()).To(Equal(a11.GetName()))
			Expect(a.GetConflicts()[0].GetName()).To(Equal(a11.GetName()))
			Expect(p.GetRequires()[0].GetName()).To(Equal(a1.GetName()))
			Expect(a.GetRequires()[0].GetName()).To(Equal(a1.GetName()))
		})
	})

	Context("BuildFormula", func() {
		It("builds empty constraints", func() {
			db := NewInMemoryDatabase(false)
			a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})

			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a1} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			f, err := a1.BuildFormula(definitions, db)
			Expect(err).ToNot(HaveOccurred())
			Expect(f).To(BeNil())
		})
		It("builds constraints correctly", func() {
			db := NewInMemoryDatabase(false)

			a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
			a21 := NewPackage("A", "1.2", []*DefaultPackage{}, []*DefaultPackage{})
			a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			a1.Requires([]*DefaultPackage{a11})
			a1.Conflicts([]*DefaultPackage{a21})

			definitions := NewInMemoryDatabase(false)
			for _, p := range []Package{a1, a21, a11} {
				_, err := definitions.CreatePackage(p)
				Expect(err).ToNot(HaveOccurred())
			}

			f, err := a1.BuildFormula(definitions, db)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(f)).To(Equal(8))
			//	Expect(f[0].String()).To(Equal("or(not(c31f5842), a4910f77)"))
			//	Expect(f[1].String()).To(Equal("or(not(c31f5842), not(a97670be))"))
		})
	})

	Context("Clone", func() {
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
		a := NewPackage("A", ">=1.0", []*DefaultPackage{a1}, []*DefaultPackage{a11})

		It("Clones correctly", func() {
			a2 := a.Clone()
			Expect(a2.GetVersion()).To(Equal(a.GetVersion()))
			Expect(a2.GetName()).To(Equal(a.GetName()))
			Expect(a2.Flagged()).To(Equal(a.Flagged()))
			Expect(a2.GetFingerPrint()).To(Equal(a.GetFingerPrint()))
			Expect(len(a2.GetConflicts())).To(Equal(len(a.GetConflicts())))
			Expect(len(a2.GetRequires())).To(Equal(len(a.GetRequires())))
			Expect(len(a2.GetRequires())).To(Equal(1))
			Expect(len(a2.GetConflicts())).To(Equal(1))
			Expect(a2.GetConflicts()[0].GetName()).To(Equal(a11.GetName()))
			Expect(a2.GetRequires()[0].GetName()).To(Equal(a1.GetName()))
		})

	})

	Context("Useflags", func() {
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		It("Adds correctly", func() {
			a1.AddUse("test")
			Expect(a1.GetUses()[0]).To(Equal("test"))
		})
		It("Removes correctly", func() {
			Expect(len(a1.GetUses())).To(Equal(1))
			a1.RemoveUse("foo")
			Expect(len(a1.GetUses())).To(Equal(1))
			a1.RemoveUse("test")
			Expect(len(a1.GetUses())).To(Equal(0))
		})
	})

	Context("Check Bump build Version", func() {
		It("Bump without build version", func() {
			a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
			err := a1.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(a1.GetVersion()).To(Equal("1.0+1"))
		})
		It("Bump 2", func() {
			p := NewPackage("A", "1.0+1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+2"))
		})

		It("Bump 3", func() {
			p := NewPackage("A", "1.0+100", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+101"))
		})

		It("Bump 4", func() {
			p := NewPackage("A", "1.0+r1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+r2"))
		})

		It("Bump 5", func() {
			p := NewPackage("A", "1.0+p1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+p2"))
		})

		It("Bump 6", func() {
			p := NewPackage("A", "1.0+pre20200315", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+pre20200315.1"))
		})

		It("Bump 7", func() {
			p := NewPackage("A", "1.0+pre20200315.1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+pre20200315.2"))
		})

		It("Bump 8", func() {
			p := NewPackage("A", "1.0+d-r1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+d-r1.1"))
		})

		It("Bump 9", func() {
			p := NewPackage("A", "1.0+p20200315.1", []*DefaultPackage{}, []*DefaultPackage{})
			err := p.BumpBuildVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetVersion()).To(Equal("1.0+p20200315.2"))
		})
	})

})
