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

package compiler_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/solver"

	. "github.com/mudler/luet/pkg/compiler"
	helpers "github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Artifact", func() {
	Context("Simple package build definition", func() {
		It("Generates a verified delta", func() {

			generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(nil, generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})
			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			lspec, ok := spec.(*LuetCompilationSpec)
			Expect(ok).To(BeTrue())

			Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
			Expect(lspec.Image).To(Equal("luet/base"))
			Expect(lspec.Seed).To(Equal("alpine"))
			tmpdir, err := ioutil.TempDir(os.TempDir(), "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpdir2, err := ioutil.TempDir(os.TempDir(), "tree2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir2) // clean up

			unpacked, err := ioutil.TempDir(os.TempDir(), "unpacked")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(unpacked) // clean up

			rootfs, err := ioutil.TempDir(os.TempDir(), "rootfs")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(rootfs) // clean up

			err = lspec.WriteBuildImageDefinition(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err := helpers.Read(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM alpine
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=enman
ENV PACKAGE_VERSION=1.4.0
ENV PACKAGE_CATEGORY=app-admin`))
			b := NewSimpleDockerBackend()
			opts := CompilerBackendOptions{
				ImageName:      "luet/base",
				SourcePath:     tmpdir,
				DockerFileName: "Dockerfile",
				Destination:    filepath.Join(tmpdir2, "output1.tar"),
			}
			Expect(b.ImageDefinitionToTar(opts)).ToNot(HaveOccurred())
			Expect(helpers.Exists(filepath.Join(tmpdir2, "output1.tar"))).To(BeTrue())
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())

			err = lspec.WriteStepImageDefinition(lspec.Image, filepath.Join(tmpdir, "LuetDockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err = helpers.Read(filepath.Join(tmpdir, "LuetDockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM luet/base
WORKDIR /luetbuild
ENV PACKAGE_NAME=enman
ENV PACKAGE_VERSION=1.4.0
ENV PACKAGE_CATEGORY=app-admin
RUN echo foo > /test
RUN echo bar > /test2`))
			opts2 := CompilerBackendOptions{
				ImageName:      "test",
				SourcePath:     tmpdir,
				DockerFileName: "LuetDockerfile",
				Destination:    filepath.Join(tmpdir, "output2.tar"),
			}
			Expect(b.ImageDefinitionToTar(opts2)).ToNot(HaveOccurred())
			Expect(helpers.Exists(filepath.Join(tmpdir, "output2.tar"))).To(BeTrue())
			diffs, err := b.Changes(opts, opts2)
			Expect(err).ToNot(HaveOccurred())

			artifacts := []ArtifactNode{}
			if os.Getenv("DOCKER_BUILDKIT") == "1" {
				artifacts = append(artifacts, ArtifactNode{Name: "/etc/resolv.conf", Size: 0})
			}
			artifacts = append(artifacts, ArtifactNode{Name: "/test", Size: 4})
			artifacts = append(artifacts, ArtifactNode{Name: "/test2", Size: 4})

			Expect(diffs).To(Equal(
				[]ArtifactLayer{{
					FromImage: filepath.Join(tmpdir2, "output1.tar"),
					ToImage:   filepath.Join(tmpdir, "output2.tar"),
					Diffs: ArtifactDiffs{
						Additions: artifacts,
					},
				}}))
			err = b.ExtractRootfs(CompilerBackendOptions{SourcePath: filepath.Join(tmpdir, "output2.tar"), Destination: rootfs}, false)
			Expect(err).ToNot(HaveOccurred())

			artifact, err := ExtractArtifactFromDelta(rootfs, filepath.Join(tmpdir, "package.tar"), diffs, 2, false, []string{}, []string{}, None)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(filepath.Join(tmpdir, "package.tar"))).To(BeTrue())
			err = helpers.Untar(artifact.GetPath(), unpacked, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(filepath.Join(unpacked, "test"))).To(BeTrue())
			Expect(helpers.Exists(filepath.Join(unpacked, "test2"))).To(BeTrue())
			content1, err := helpers.Read(filepath.Join(unpacked, "test"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("foo\n"))
			content2, err := helpers.Read(filepath.Join(unpacked, "test2"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content2).To(Equal("bar\n"))

			err = artifact.Hash()
			Expect(err).ToNot(HaveOccurred())
			err = artifact.Verify()
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.CopyFile(filepath.Join(tmpdir, "output2.tar"), filepath.Join(tmpdir, "package.tar"))).ToNot(HaveOccurred())

			err = artifact.Verify()
			Expect(err).To(HaveOccurred())
		})

	})
})
