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

package gentoo_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	pkg "github.com/mudler/luet/pkg/package"
	. "github.com/mudler/luet/pkg/tree/builder/gentoo"
)

type FakeParser struct {
}

func (f *FakeParser) ScanEbuild(path string) ([]pkg.Package, error) {
	return []pkg.Package{&pkg.DefaultPackage{Name: path}}, nil
}

var _ = Describe("GentooBuilder", func() {

	Context("Simple test", func() {
		for _, dbType := range []MemoryDB{InMemory, BoltDB} {
			It("parses correctly deps", func() {
				gb := NewGentooBuilder(&FakeParser{}, 20, dbType)
				tree, err := gb.Generate("../../../../tests/fixtures/overlay")
				defer func() {
					Expect(tree.Clean()).ToNot(HaveOccurred())
				}()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tree.GetPackages())).To(Equal(10))
			})
		}
	})

	Context("Parse ebuild1", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/overlay/app-crypt/pinentry-gnome/pinentry-gnome-1.0.0-r2.ebuild")
		It("parses correctly deps", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(pkgs[0].GetLicense()).To(Equal("GPL-2"))
			Expect(pkgs[0].GetDescription()).To(Equal("GNOME 3 frontend for pinentry"))
		})
	})

	Context("Parse ebuild2", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/mod_dav_svn-1.12.2.ebuild")

		It("Parsing ebuild2", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(pkgs[0].GetLicense()).To(Equal("Subversion"))
			Expect(pkgs[0].GetDescription()).To(Equal("Subversion WebDAV support"))
		})
	})

	Context("Parse ebuild3", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/linux-sources-1.ebuild")

		It("Check parsing of the ebuild3", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(0))
			Expect(pkgs[0].GetLicense()).To(Equal(""))
			Expect(pkgs[0].GetDescription()).To(Equal("Virtual for Linux kernel sources"))
		})
	})

	Context("Parse ebuild4", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/sabayon-mce-1.1-r5.ebuild")

		It("Check parsing of the ebuild4", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(2))
			Expect(pkgs[0].GetLicense()).To(Equal("GPL-2"))
			Expect(pkgs[0].GetDescription()).To(Equal("Sabayon Linux Media Center Infrastructure"))
		})
	})

	Context("Parse ebuild5", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/libreoffice-l10n-meta-6.2.8.2.ebuild")

		It("Check parsing of the ebuild5", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(146))
			Expect(pkgs[0].GetLicense()).To(Equal("LGPL-2"))
			Expect(pkgs[0].GetDescription()).To(Equal("LibreOffice.org localisation meta-package"))
		})
	})

	Context("Parse ebuild6", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/pkgs-checker-0.2.0.ebuild")

		It("Check parsing of the ebuild6", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(0))
			Expect(pkgs[0].GetLicense()).To(Equal("GPL-3"))
			Expect(pkgs[0].GetDescription()).To(Equal("Sabayon Packages Checker"))
		})
	})

	Context("Parse ebuild7", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/calamares-sabayon-base-modules-1.15.ebuild")

		It("Check parsing of the ebuild7", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(2))
			Expect(pkgs[0].GetLicense()).To(Equal("CC-BY-SA-4.0"))
			Expect(pkgs[0].GetDescription()).To(Equal("Sabayon Official Calamares base modules"))
		})
	})

	Context("Parse ebuild8", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/subversion-1.12.0.ebuild")

		It("Check parsing of the ebuild8", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(25))
			Expect(pkgs[0].GetLicense()).To(Equal("Subversion GPL-2"))
			Expect(pkgs[0].GetDescription()).To(Equal("Advanced version control system"))
		})
	})

	Context("Parse ebuild9", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/kodi-raspberrypi-16.0.ebuild")

		PIt("Check parsing of the ebuild9", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(66))
			Expect(pkgs[0].GetLicense()).To(Equal("GPL-2"))
			Expect(pkgs[0].GetDescription()).To(Equal("Kodi is a free and open source media-player and entertainment hub"))
		})
	})

	Context("Parse ebuild10", func() {
		parser := &SimpleEbuildParser{}
		pkgs, err := parser.ScanEbuild("../../../../tests/fixtures/parser/tango-icon-theme-0.8.90-r1.ebuild")

		It("Check parsing of the ebuild10", func() {
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("PKG ", pkgs[0])
			Expect(len(pkgs[0].GetRequires())).To(Equal(2))
			Expect(pkgs[0].GetLicense()).To(Equal("public-domain"))
			Expect(pkgs[0].GetDescription()).To(Equal("SVG and PNG icon theme from the Tango project"))
		})
	})

})
