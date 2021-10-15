// Copyright © 2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package client_test

import (
	. "github.com/mudler/luet/pkg/api/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client CLI API", func() {
	Context("Reads a package tree from the luet CLI", func() {
		It("Correctly detect packages", func() {
			t, err := TreePackages("../../../tests/fixtures/alpine")
			Expect(err).ToNot(HaveOccurred())
			Expect(t).ToNot(BeNil())
			Expect(len(t.Packages)).To(Equal(1))
			Expect(t.Packages[0].Name).To(Equal("alpine"))
			Expect(t.Packages[0].Category).To(Equal("seed"))
			Expect(t.Packages[0].Version).To(Equal("1.0"))
			Expect(t.Packages[0].ImageAvailable("foo")).To(BeFalse())
			Expect(t.Packages[0].Equal(t.Packages[0])).To(BeTrue())
			Expect(t.Packages[0].Equal(Package{})).To(BeFalse())
			Expect(t.Packages[0].EqualNoV(Package{Name: "alpine", Category: "seed"})).To(BeTrue())
			Expect(t.Packages[0].EqualS("seed/alpine")).To(BeTrue())
			Expect(t.Packages[0].EqualS("seed/alpinev")).To(BeFalse())
			Expect(t.Packages[0].EqualSV("seed/alpine@1.0")).To(BeTrue())
			Expect(t.Packages[0].Image("foo")).To(Equal("foo:alpine-seed-1.0"))
			Expect(Packages(t.Packages).Exist(t.Packages[0])).To(BeTrue())
			Expect(Packages(t.Packages).Exist(Package{})).To(BeFalse())

		})
	})
})
