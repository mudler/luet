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

package types_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/types"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Context("Inits paths", func() {
		t, _ := ioutil.TempDir("", "tests")
		defer os.RemoveAll(t)
		c := &types.LuetConfig{
			System: types.LuetSystemConfig{
				Rootfs:        t,
				PkgsCachePath: "foo",
				DatabasePath:  "baz",
			}}
		It("sets default", func() {
			err := c.Init()
			Expect(err).ToNot(HaveOccurred())
			Expect(c.System.Rootfs).To(Equal(t))
			Expect(c.System.PkgsCachePath).To(Equal(filepath.Join(t, "baz", "foo")))
			Expect(c.System.DatabasePath).To(Equal(filepath.Join(t, "baz")))
		})
	})

	Context("Load Repository1", func() {
		var ctx *context.Context
		BeforeEach(func() {
			ctx = context.NewContext(context.WithConfig(&types.LuetConfig{
				RepositoriesConfDir: []string{
					"../../../../tests/fixtures/repos.conf.d",
				},
			}))
			ctx.Config.Init()
		})

		It("Check Load Repository 1", func() {
			Expect(len(ctx.GetConfig().SystemRepositories)).Should(Equal(2))
			Expect(ctx.GetConfig().SystemRepositories[0].Name).Should(Equal("test1"))
			Expect(ctx.GetConfig().SystemRepositories[0].Priority).Should(Equal(999))
			Expect(ctx.GetConfig().SystemRepositories[0].Type).Should(Equal("disk"))
			Expect(len(ctx.GetConfig().SystemRepositories[0].Urls)).Should(Equal(1))
			Expect(ctx.GetConfig().SystemRepositories[0].Urls[0]).Should(Equal("tests/repos/test1"))
		})

		It("Chec Load Repository 2", func() {
			Expect(len(ctx.GetConfig().SystemRepositories)).Should(Equal(2))
			Expect(ctx.GetConfig().SystemRepositories[1].Name).Should(Equal("test2"))
			Expect(ctx.GetConfig().SystemRepositories[1].Priority).Should(Equal(1000))
			Expect(ctx.GetConfig().SystemRepositories[1].Type).Should(Equal("disk"))
			Expect(len(ctx.GetConfig().SystemRepositories[1].Urls)).Should(Equal(1))
			Expect(ctx.GetConfig().SystemRepositories[1].Urls[0]).Should(Equal("tests/repos/test2"))
		})
	})

	Context("Simple temporary directory creation", func() {
		ctx := context.NewContext(context.WithConfig(&types.LuetConfig{
			System: types.LuetSystemConfig{
				TmpDirBase: os.TempDir() + "/tmpluet",
			},
		}))

		BeforeEach(func() {
			ctx = context.NewContext(context.WithConfig(&types.LuetConfig{
				System: types.LuetSystemConfig{
					TmpDirBase: os.TempDir() + "/tmpluet",
				},
			}))

		})

		It("Create Temporary directory", func() {
			tmpDir, err := ctx.TempDir("test1")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(tmpDir, filepath.Join(os.TempDir(), "tmpluet"))).To(BeTrue())
			Expect(fileHelper.Exists(tmpDir)).To(BeTrue())

			defer os.RemoveAll(tmpDir)
		})

		It("Create Temporary file", func() {
			tmpFile, err := ctx.TempFile("testfile1")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(tmpFile.Name(), filepath.Join(os.TempDir(), "tmpluet"))).To(BeTrue())
			Expect(fileHelper.Exists(tmpFile.Name())).To(BeTrue())

			defer os.Remove(tmpFile.Name())
		})

	})

})
