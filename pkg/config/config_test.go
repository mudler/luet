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
	"os"
	"path/filepath"
	"strings"

	config "github.com/mudler/luet/pkg/config"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Context("Simple temporary directory creation", func() {

		It("Create Temporary directory", func() {
			// PRE: tmpdir_base contains default value.

			tmpDir, err := config.LuetCfg.GetSystem().TempDir("test1")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(tmpDir, filepath.Join(os.TempDir(), "tmpluet"))).To(BeTrue())
			Expect(fileHelper.Exists(tmpDir)).To(BeTrue())

			defer os.RemoveAll(tmpDir)
		})

		It("Create Temporary file", func() {
			// PRE: tmpdir_base contains default value.

			tmpFile, err := config.LuetCfg.GetSystem().TempFile("testfile1")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(tmpFile.Name(), filepath.Join(os.TempDir(), "tmpluet"))).To(BeTrue())
			Expect(fileHelper.Exists(tmpFile.Name())).To(BeTrue())

			defer os.Remove(tmpFile.Name())
		})

		It("Config1", func() {
			cfg := config.LuetCfg

			cfg.GetLogging().Color = false
			Expect(cfg.GetLogging().Color).To(BeFalse())
		})

	})

})
