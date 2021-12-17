// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>
//                       Daniele Rondina <geaaru@sabayonlinux.org>
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

package config_test

import (
	config "github.com/mudler/luet/pkg/api/core/config"
	"github.com/mudler/luet/pkg/api/core/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Context("Test config protect", func() {

		It("Protect1", func() {
			ctx := context.NewContext()
			files := []string{
				"etc/foo/my.conf",
				"usr/bin/foo",
				"usr/share/doc/foo.md",
			}

			cp := config.NewConfigProtect("/etc")
			cp.Map(files, ctx.Config.ConfigProtectConfFiles)

			Expect(cp.Protected("etc/foo/my.conf")).To(BeTrue())
			Expect(cp.Protected("/etc/foo/my.conf")).To(BeTrue())
			Expect(cp.Protected("usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/share/doc/foo.md")).To(BeFalse())

			Expect(cp.GetProtectFiles(false)).To(Equal(
				[]string{
					"etc/foo/my.conf",
				},
			))

			Expect(cp.GetProtectFiles(true)).To(Equal(
				[]string{
					"/etc/foo/my.conf",
				},
			))
		})

		It("Protect2", func() {
			ctx := context.NewContext()

			files := []string{
				"etc/foo/my.conf",
				"usr/bin/foo",
				"usr/share/doc/foo.md",
			}

			cp := config.NewConfigProtect("")
			cp.Map(files, ctx.Config.ConfigProtectConfFiles)

			Expect(cp.Protected("etc/foo/my.conf")).To(BeFalse())
			Expect(cp.Protected("/etc/foo/my.conf")).To(BeFalse())
			Expect(cp.Protected("usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/share/doc/foo.md")).To(BeFalse())

			Expect(cp.GetProtectFiles(false)).To(Equal(
				[]string{},
			))

			Expect(cp.GetProtectFiles(true)).To(Equal(
				[]string{},
			))
		})

		It("Protect3: Annotation dir without initial slash", func() {
			ctx := context.NewContext()

			files := []string{
				"etc/foo/my.conf",
				"usr/bin/foo",
				"usr/share/doc/foo.md",
			}

			cp := config.NewConfigProtect("etc")
			cp.Map(files, ctx.Config.ConfigProtectConfFiles)

			Expect(cp.Protected("etc/foo/my.conf")).To(BeTrue())
			Expect(cp.Protected("/etc/foo/my.conf")).To(BeTrue())
			Expect(cp.Protected("usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/bin/foo")).To(BeFalse())
			Expect(cp.Protected("/usr/share/doc/foo.md")).To(BeFalse())

			Expect(cp.GetProtectFiles(false)).To(Equal(
				[]string{
					"etc/foo/my.conf",
				},
			))

			Expect(cp.GetProtectFiles(true)).To(Equal(
				[]string{
					"/etc/foo/my.conf",
				},
			))
		})

	})

})
