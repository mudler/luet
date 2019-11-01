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

	. "github.com/mudler/luet/pkg/tree/builder/gentoo"
)

var _ = Describe("GentooBuilder", func() {

	Context("Simple test", func() {
		It("parses correctly deps", func() {
			gb := NewGentooBuilder(&SimpleEbuildParser{}, 20)
			tree, err := gb.Generate("../../../../tests/fixtures/overlay")
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
			}()

			Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))

			for _, pid := range tree.GetPackageSet().GetPackages() {
				p, err := tree.GetPackageSet().GetPackage(pid)
				Expect(err).ToNot(HaveOccurred())
				Expect(p.GetName()).To(ContainSubstring("pinentry"))
				//	Expect(p.GetVersion()).To(ContainSubstring("1."))
			}

		})
	})

})
