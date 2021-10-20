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

package backend_test

import (
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/mudler/luet/pkg/compiler"
	. "github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/mudler/luet/pkg/compiler/backend"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"io/ioutil"
	"os"
	"path/filepath"

	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker backend", func() {
	Context("Simple Docker backend satisfies main interface functionalities", func() {
		ctx := types.NewContext()
		It("Builds and generate tars", func() {
			generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			cc := NewLuetCompiler(nil, generalRecipe.GetDatabase())
			lspec, err := cc.FromPackage(&pkg.DefaultPackage{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
			Expect(lspec.Image).To(Equal("luet/base"))
			Expect(lspec.Seed).To(Equal("alpine"))
			tmpdir, err := ioutil.TempDir(os.TempDir(), "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpdir2, err := ioutil.TempDir(os.TempDir(), "tree2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir2) // clean up

			err = lspec.WriteBuildImageDefinition(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err := fileHelper.Read(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM alpine
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=enman
ENV PACKAGE_VERSION=1.4.0
ENV PACKAGE_CATEGORY=app-admin`))
			b := NewSimpleDockerBackend(ctx)
			opts := backend.Options{
				ImageName:      "luet/base",
				SourcePath:     tmpdir,
				DockerFileName: "Dockerfile",
				Destination:    filepath.Join(tmpdir2, "output1.tar"),
			}

			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(tmpdir2, "output1.tar"))).To(BeTrue())

			err = lspec.WriteStepImageDefinition(lspec.Image, filepath.Join(tmpdir, "LuetDockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err = fileHelper.Read(filepath.Join(tmpdir, "LuetDockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM luet/base
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=enman
ENV PACKAGE_VERSION=1.4.0
ENV PACKAGE_CATEGORY=app-admin
RUN echo foo > /test
RUN echo bar > /test2`))
			opts2 := backend.Options{
				ImageName:      "test",
				SourcePath:     tmpdir,
				DockerFileName: "LuetDockerfile",
				Destination:    filepath.Join(tmpdir, "output2.tar"),
			}

			Expect(b.BuildImage(opts2)).ToNot(HaveOccurred())
			Expect(b.ExportImage(opts2)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(tmpdir, "output2.tar"))).To(BeTrue())

			artifacts := []artifact.ArtifactNode{{
				Name: "/luetbuild/LuetDockerfile",
				Size: 175,
			}}
			if os.Getenv("DOCKER_BUILDKIT") == "1" {
				artifacts = append(artifacts, artifact.ArtifactNode{Name: "/etc/resolv.conf", Size: 0})
			}
			artifacts = append(artifacts, artifact.ArtifactNode{Name: "/test", Size: 4})
			artifacts = append(artifacts, artifact.ArtifactNode{Name: "/test2", Size: 4})

			Expect(compiler.GenerateChanges(ctx, b, opts, opts2)).To(Equal(
				[]artifact.ArtifactLayer{{
					FromImage: "luet/base",
					ToImage:   "test",
					Diffs: artifact.ArtifactDiffs{
						Additions: artifacts,
					},
				}}))

			opts2 = backend.Options{
				ImageName:      "test",
				SourcePath:     tmpdir,
				DockerFileName: "LuetDockerfile",
				Destination:    filepath.Join(tmpdir, "output3.tar"),
			}

			Expect(b.ImageDefinitionToTar(opts2)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(tmpdir, "output3.tar"))).To(BeTrue())
			Expect(b.ImageExists(opts2.ImageName)).To(BeFalse())
		})

		It("Detects available images", func() {
			b := NewSimpleDockerBackend(ctx)
			Expect(b.ImageAvailable("quay.io/mocaccino/extra")).To(BeTrue())
			Expect(b.ImageAvailable("ubuntu:20.10")).To(BeTrue())
			Expect(b.ImageAvailable("igjo5ijgo25nho52")).To(BeFalse())
		})
	})
})
