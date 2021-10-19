// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

	. "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {
	Context("Repository detects underlying arch", func() {
		It("is enabled if arch is matching", func() {
			r := LuetRepository{Arch: runtime.GOARCH}
			Expect(r.Enabled()).To(BeTrue())
		})
		It("is disabled if arch is NOT matching", func() {
			r := LuetRepository{Arch: "foo"}
			Expect(r.Enabled()).To(BeFalse())
		})
		It("is enabled if arch is NOT matching and enabled is true", func() {
			r := LuetRepository{Arch: "foo", Enable: true}
			Expect(r.Enabled()).To(BeTrue())
		})
		It("enabled is true", func() {
			r := LuetRepository{Enable: true}
			Expect(r.Enabled()).To(BeTrue())
		})
		It("enabled is false", func() {
			r := LuetRepository{Enable: false}
			Expect(r.Enabled()).To(BeFalse())
		})
	})
})
