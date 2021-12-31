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

package cmd_helpers_test

import (
	. "github.com/mudler/luet/cmd/helpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI Helpers", func() {
	Context("Can parse package strings correctly", func() {
		It("accept single package names", func() {
			pack, err := ParsePackageStr("foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal(""))
			Expect(pack.GetVersion()).To(Equal(">=0"))
		})
		It("accept unversioned packages with category", func() {
			pack, err := ParsePackageStr("cat/foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal(">=0"))
		})
		It("accept versioned packages with category", func() {
			pack, err := ParsePackageStr("cat/foo@1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal("1.1"))
		})
		It("accept versioned ranges with category", func() {
			pack, err := ParsePackageStr("cat/foo@>=1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal(">=1.1"))
		})
		It("accept gentoo regex parsing without versions", func() {
			pack, err := ParsePackageStr("=cat/foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal(">=0"))
		})
		It("accept gentoo regex parsing with versions", func() {
			pack, err := ParsePackageStr("=cat/foo-1.2")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal("1.2"))
		})

		It("accept gentoo regex parsing with with condition", func() {
			pack, err := ParsePackageStr(">=cat/foo-1.2")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal(">=1.2"))
		})

		It("accept gentoo regex parsing with with condition2", func() {
			pack, err := ParsePackageStr("<cat/foo-1.2")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal("<1.2"))
		})

		It("accept gentoo regex parsing with with condition3", func() {
			pack, err := ParsePackageStr(">cat/foo-1.2")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal(">1.2"))
		})

		It("accept gentoo regex parsing with with condition4", func() {
			pack, err := ParsePackageStr("<=cat/foo-1.2")
			Expect(err).ToNot(HaveOccurred())
			Expect(pack.GetName()).To(Equal("foo"))
			Expect(pack.GetCategory()).To(Equal("cat"))
			Expect(pack.GetVersion()).To(Equal("<=1.2"))
		})
	})
})
