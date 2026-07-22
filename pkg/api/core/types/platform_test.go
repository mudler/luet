// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types_test

import (
	"runtime"

	types "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Platform", func() {
	Context("ParsePlatform", func() {
		It("parses os/arch", func() {
			p, err := types.ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.OS).To(Equal("linux"))
			Expect(p.Arch).To(Equal("amd64"))
			Expect(p.Variant).To(Equal(""))
		})

		It("parses os/arch/variant", func() {
			p, err := types.ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.OS).To(Equal("linux"))
			Expect(p.Arch).To(Equal("arm"))
			Expect(p.Variant).To(Equal("v7"))
		})

		It("rejects the empty string", func() {
			_, err := types.ParsePlatform("")
			Expect(err).To(HaveOccurred())
		})

		It("rejects a single component", func() {
			_, err := types.ParsePlatform("amd64")
			Expect(err).To(HaveOccurred())
		})

		It("rejects too many components", func() {
			_, err := types.ParsePlatform("linux/arm/v7/extra")
			Expect(err).To(HaveOccurred())
		})

		It("rejects empty components", func() {
			_, err := types.ParsePlatform("linux//v7")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("String", func() {
		It("round-trips os/arch", func() {
			p, err := types.ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.String()).To(Equal("linux/amd64"))
		})

		It("round-trips os/arch/variant", func() {
			p, err := types.ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.String()).To(Equal("linux/arm/v7"))
		})

		It("renders the zero value as the empty string", func() {
			Expect(types.Platform{}.String()).To(Equal(""))
		})
	})

	Context("IsZero", func() {
		It("is true for the zero value", func() {
			Expect(types.Platform{}.IsZero()).To(BeTrue())
		})

		It("is false for a parsed platform", func() {
			p, err := types.ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.IsZero()).To(BeFalse())
		})
	})

	Context("HostPlatform", func() {
		It("reports the running host", func() {
			p := types.HostPlatform()
			Expect(p.OS).To(Equal(runtime.GOOS))
			Expect(p.Arch).To(Equal(runtime.GOARCH))
			Expect(p.IsZero()).To(BeFalse())
		})
	})
})
