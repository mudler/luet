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
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/compiler"
	. "github.com/mudler/luet/pkg/compiler/backend"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/mudler/luet/pkg/compiler/types/artifact"
	compression "github.com/mudler/luet/pkg/compiler/types/compression"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"

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

			err := generalRecipe.Load("../../../../tests/fixtures/buildtree")
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

			unpacked, err := ioutil.TempDir(os.TempDir(), "unpacked")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(unpacked) // clean up

			rootfs, err := ioutil.TempDir(os.TempDir(), "rootfs")
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
			b := NewSimpleDockerBackend()
			opts := backend.Options{
				ImageName:      "luet/base",
				SourcePath:     tmpdir,
				DockerFileName: "Dockerfile",
				Destination:    filepath.Join(tmpdir2, "output1.tar"),
			}
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(tmpdir2, "output1.tar"))).To(BeTrue())
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
			Expect(fileHelper.Exists(filepath.Join(tmpdir, "output2.tar"))).To(BeTrue())
			diffs, err := compiler.GenerateChanges(b, opts, opts2)
			Expect(err).ToNot(HaveOccurred())

			artifacts := []ArtifactNode{{
				Name: "/luetbuild/LuetDockerfile",
				Size: 175,
			}}
			if os.Getenv("DOCKER_BUILDKIT") == "1" {
				artifacts = append(artifacts, ArtifactNode{Name: "/etc/resolv.conf", Size: 0})
			}
			artifacts = append(artifacts, ArtifactNode{Name: "/test", Size: 4})
			artifacts = append(artifacts, ArtifactNode{Name: "/test2", Size: 4})

			Expect(diffs).To(Equal(
				[]ArtifactLayer{{
					FromImage: "luet/base",
					ToImage:   "test",
					Diffs: ArtifactDiffs{
						Additions: artifacts,
					},
				}}))
			err = b.ExtractRootfs(backend.Options{ImageName: "test", Destination: rootfs}, false)
			Expect(err).ToNot(HaveOccurred())

			a, err := ExtractArtifactFromDelta(rootfs, filepath.Join(tmpdir, "package.tar"), diffs, 2, false, []string{}, []string{}, compression.None)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(tmpdir, "package.tar"))).To(BeTrue())
			err = helpers.Untar(a.Path, unpacked, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(filepath.Join(unpacked, "test"))).To(BeTrue())
			Expect(fileHelper.Exists(filepath.Join(unpacked, "test2"))).To(BeTrue())
			content1, err := fileHelper.Read(filepath.Join(unpacked, "test"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("foo\n"))
			content2, err := fileHelper.Read(filepath.Join(unpacked, "test2"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content2).To(Equal("bar\n"))

			err = a.Hash()
			Expect(err).ToNot(HaveOccurred())
			err = a.Verify()
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.CopyFile(filepath.Join(tmpdir, "output2.tar"), filepath.Join(tmpdir, "package.tar"))).ToNot(HaveOccurred())

			err = a.Verify()
			Expect(err).To(HaveOccurred())
		})

		It("Generates packages images", func() {
			b := NewSimpleDockerBackend()
			imageprefix := "foo/"
			testString := []byte(`funky test data`)

			tmpdir, err := ioutil.TempDir(os.TempDir(), "artifact")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpWork, err := ioutil.TempDir(os.TempDir(), "artifact2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpWork) // clean up

			Expect(os.MkdirAll(filepath.Join(tmpdir, "foo", "bar"), os.ModePerm)).ToNot(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(tmpdir, "test"), testString, 0644)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(tmpdir, "foo", "bar", "test"), testString, 0644)
			Expect(err).ToNot(HaveOccurred())

			a := NewPackageArtifact(filepath.Join(tmpWork, "fake.tar"))
			a.CompileSpec = &compilerspec.LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "foo", Version: "1.0"}}

			err = a.Compress(tmpdir, 1)
			Expect(err).ToNot(HaveOccurred())
			resultingImage := imageprefix + "foo--1.0"
			opts, err := a.GenerateFinalImage(resultingImage, b, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.ImageName).To(Equal(resultingImage))

			Expect(b.ImageExists(resultingImage)).To(BeTrue())

			result, err := ioutil.TempDir(os.TempDir(), "result")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(result) // clean up

			err = b.ExtractRootfs(backend.Options{ImageName: resultingImage, Destination: result}, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := ioutil.ReadFile(filepath.Join(result, "test"))
			Expect(err).ToNot(HaveOccurred())

			Expect(content).To(Equal(testString))

			content, err = ioutil.ReadFile(filepath.Join(result, "foo", "bar", "test"))
			Expect(err).ToNot(HaveOccurred())

			Expect(content).To(Equal(testString))
		})

		It("Generates empty packages images", func() {
			b := NewSimpleDockerBackend()
			imageprefix := "foo/"

			tmpdir, err := ioutil.TempDir(os.TempDir(), "artifact")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpWork, err := ioutil.TempDir(os.TempDir(), "artifact2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpWork) // clean up

			a := NewPackageArtifact(filepath.Join(tmpWork, "fake.tar"))
			a.CompileSpec = &compilerspec.LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "foo", Version: "1.0"}}

			err = a.Compress(tmpdir, 1)
			Expect(err).ToNot(HaveOccurred())
			resultingImage := imageprefix + "foo--1.0"
			opts, err := a.GenerateFinalImage(resultingImage, b, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.ImageName).To(Equal(resultingImage))

			Expect(b.ImageExists(resultingImage)).To(BeTrue())

			result, err := ioutil.TempDir(os.TempDir(), "result")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(result) // clean up

			err = b.ExtractRootfs(backend.Options{ImageName: resultingImage, Destination: result}, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(fileHelper.DirectoryIsEmpty(result)).To(BeFalse())
			content, err := ioutil.ReadFile(filepath.Join(result, ".virtual"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(content)).To(Equal(""))
		})

		It("Retrieves uncompressed name", func() {
			a := NewPackageArtifact("foo.tar.gz")
			a.CompressionType = (compression.GZip)
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))

			a = NewPackageArtifact("foo.tar.zst")
			a.CompressionType = compression.Zstandard
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))

			a = NewPackageArtifact("foo.tar")
			a.CompressionType = compression.None
			Expect(a.GetUncompressedName()).To(Equal("foo.tar"))
		})
	})
})
