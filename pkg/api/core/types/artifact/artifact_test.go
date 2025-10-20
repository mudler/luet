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

package artifact_test

import (
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/mudler/luet/pkg/compiler"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/image"
	. "github.com/mudler/luet/pkg/api/core/types/artifact"
	backend "github.com/mudler/luet/pkg/compiler/backend"

	. "github.com/mudler/luet/pkg/compiler"
	pkg "github.com/mudler/luet/pkg/database"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Artifact", func() {
	Context("Simple package build definition", func() {
		ctx := context.NewContext()
		It("Generates a verified delta", func() {

			generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../../../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			cc := NewLuetCompiler(nil, generalRecipe.GetDatabase(), compiler.WithContext(context.NewContext()))
			lspec, err := cc.FromPackage(&types.Package{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
			Expect(lspec.Image).To(Equal("luet/base"))
			Expect(lspec.Seed).To(Equal("alpine"))
			tmpdir, err := os.MkdirTemp(os.TempDir(), "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpdir2, err := os.MkdirTemp(os.TempDir(), "tree2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir2) // clean up

			unpacked, err := os.MkdirTemp(os.TempDir(), "unpacked")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(unpacked) // clean up

			rootfs, err := os.MkdirTemp(os.TempDir(), "rootfs")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(rootfs) // clean up

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
			b := backend.NewSimpleDockerBackend(ctx)
			opts := backend.Options{
				ImageName:      "luet/base",
				SourcePath:     tmpdir,
				DockerFileName: "Dockerfile",
				Destination:    filepath.Join(tmpdir2, "output1.tar"),
			}
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(filepath.Join(tmpdir2, "output1.tar")).To(BeARegularFile())
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())

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

		})

		It("Generates packages images", func() {
			b := backend.NewSimpleDockerBackend(ctx)
			imageprefix := "foo/"
			testString := []byte(`funky test data`)

			tmpdir, err := os.MkdirTemp(os.TempDir(), "artifact")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpWork, err := os.MkdirTemp(os.TempDir(), "artifact2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpWork) // clean up

			Expect(os.MkdirAll(filepath.Join(tmpdir, "foo", "bar"), os.ModePerm)).ToNot(HaveOccurred())

			err = os.WriteFile(filepath.Join(tmpdir, "test"), testString, 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(filepath.Join(tmpdir, "foo", "bar", "test"), testString, 0644)
			Expect(err).ToNot(HaveOccurred())

			a := NewPackageArtifact(filepath.Join(tmpWork, "fake.tar"))
			a.CompileSpec = &types.LuetCompilationSpec{Package: &types.Package{Name: "foo", Version: "1.0"}}

			err = a.Compress(tmpdir, 1)
			Expect(err).ToNot(HaveOccurred())
			resultingImage := imageprefix + "foo--1.0"
			err = a.GenerateFinalImage(ctx, resultingImage, b, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(b.ImageExists(resultingImage)).To(BeTrue())

			result, err := os.MkdirTemp(os.TempDir(), "result")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(result) // clean up

			img, err := b.ImageReference(resultingImage, true)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = image.ExtractTo(
				ctx,
				img,
				result,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filepath.Join(result, "test"))
			Expect(err).ToNot(HaveOccurred())

			Expect(content).To(Equal(testString))

			content, err = os.ReadFile(filepath.Join(result, "foo", "bar", "test"))
			Expect(err).ToNot(HaveOccurred())

			Expect(content).To(Equal(testString))
		})

		It("Generates empty packages images", func() {
			b := backend.NewSimpleDockerBackend(ctx)
			imageprefix := "foo/"

			tmpdir, err := os.MkdirTemp(os.TempDir(), "artifact")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpWork, err := os.MkdirTemp(os.TempDir(), "artifact2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpWork) // clean up

			a := NewPackageArtifact(filepath.Join(tmpWork, "fake.tar"))
			a.CompileSpec = &types.LuetCompilationSpec{Package: &types.Package{Name: "foo", Version: "1.0"}}

			err = a.Compress(tmpdir, 1)
			Expect(err).ToNot(HaveOccurred())
			resultingImage := imageprefix + "foo--1.0"
			err = a.GenerateFinalImage(ctx, resultingImage, b, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(b.ImageExists(resultingImage)).To(BeTrue())

			result, err := os.MkdirTemp(os.TempDir(), "result")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(result) // clean up

			img, err := b.ImageReference(resultingImage, false)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = image.ExtractTo(
				ctx,
				img,
				result,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(fileHelper.DirectoryIsEmpty(result)).To(BeTrue())
		})

		It("Retrieves uncompressed name", func() {
			a := NewPackageArtifact("foo.tar.gz")
			a.CompressionType = (types.GZip)
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))

			a = NewPackageArtifact("foo.tar.zst")
			a.CompressionType = types.Zstandard
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))

			a = NewPackageArtifact("foo.tar")
			a.CompressionType = types.None
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))
		})
	})
})
