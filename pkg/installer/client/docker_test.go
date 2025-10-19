// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	. "github.com/mudler/luet/pkg/installer/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This test expect that the repository defined in UNIT_TEST_DOCKER_IMAGE is in zstd format.
// the repository is built by the 01_simple_docker.sh integration test fileHelper.
// This test also require root. At the moment, unpacking docker images with 'img' requires root permission to
// mount/unmount layers.
var _ = Describe("Docker client", func() {
	Context("With repository", func() {
		ctx := context.NewContext()

		repoImage := os.Getenv("UNIT_TEST_DOCKER_IMAGE")
		var repoURL []string
		var c *DockerClient
		BeforeEach(func() {
			if repoImage == "" {
				Skip("UNIT_TEST_DOCKER_IMAGE not specified")
			}
			repoURL = []string{repoImage}
			c = NewDockerClient(RepoData{Urls: repoURL}, ctx)
		})

		It("Downloads single files", func() {
			f, err := c.DownloadFile("repository.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Read(f)).To(ContainSubstring("Test Repo"))
			os.RemoveAll(f)
		})

		It("Downloads artifacts", func() {
			f, err := c.DownloadArtifact(&artifact.PackageArtifact{
				Path: "test.tar",
				CompileSpec: &types.LuetCompilationSpec{
					Package: &types.Package{
						Name:     "c",
						Category: "test",
						Version:  "1.0",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			tmpdir, err := os.MkdirTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(f.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Read(filepath.Join(tmpdir, "c"))).To(Equal("c\n"))
			Expect(fileHelper.Read(filepath.Join(tmpdir, "cd"))).To(Equal("c\n"))
			os.RemoveAll(f.Path)
		})
	})
})
