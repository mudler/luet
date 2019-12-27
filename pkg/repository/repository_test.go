// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package repository_test

import (
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/repository"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	viper "github.com/spf13/viper"
)

var _ = Describe("Repository", func() {
	Context("Load Repository1", func() {
		cfg := NewLuetConfig(viper.New())
		cfg.RepositoriesConfDir = []string{
			"../../tests/fixtures/repos.conf.d",
		}
		err := LoadRepositories(cfg)

		It("Chec Load Repository 1", func() {
			Expect(err).Should(BeNil())
			Expect(len(cfg.SystemRepositories)).Should(Equal(1))
			Expect(cfg.SystemRepositories[0].Name).Should(Equal("test1"))
			Expect(cfg.SystemRepositories[0].Priority).Should(Equal(999))
			Expect(cfg.SystemRepositories[0].Type).Should(Equal("disk"))
			Expect(len(cfg.SystemRepositories[0].Urls)).Should(Equal(1))
			Expect(cfg.SystemRepositories[0].Urls[0]).Should(Equal("tests/repos/test1"))
		})
	})
})
