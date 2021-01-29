// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>
//                       David Cassany <dcassany@suse.com>
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

package helpers_test

import (
	. "github.com/mudler/luet/pkg/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helpers", func() {
	Context("StripRegistryFromImage", func() {
		It("Strips the domain name", func() {
			out := StripRegistryFromImage("valid.domain.org/base/image:tag")
			Expect(out).To(Equal("base/image:tag"))
		})
		It("Strips the domain name when port is included", func() {
			out := StripRegistryFromImage("valid.domain.org:5000/base/image:tag")
			Expect(out).To(Equal("base/image:tag"))
		})
		It("Does not strip the domain name", func() {
			out := StripRegistryFromImage("not-a-domain/base/image:tag")
			Expect(out).To(Equal("not-a-domain/base/image:tag"))
		})
		It("Does not strip the domain name on invalid domains", func() {
			out := StripRegistryFromImage("-invaliddomain.org/base/image:tag")
			Expect(out).To(Equal("-invaliddomain.org/base/image:tag"))
		})
	})
})
