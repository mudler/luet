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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	pkg "github.com/mudler/luet/pkg/package"
	. "github.com/mudler/luet/pkg/tree/builder/gentoo"
)

type FakeParser struct {
}

func (f *FakeParser) ScanEbuild(path string, t pkg.Tree) ([]pkg.Package, error) {
	return []pkg.Package{&pkg.DefaultPackage{Name: path}}, nil
}

var _ = Describe("GentooBuilder", func() {

	Context("Simple test", func() {
		for _, dbType := range []MemoryDB{InMemory, BoltDB} {
			It("parses correctly deps", func() {
				gb := NewGentooBuilder(&FakeParser{}, 20, dbType)
				tree, err := gb.Generate("../../../../tests/fixtures/overlay")
				defer func() {
					Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
				}()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))
			})
		}
	})

})
