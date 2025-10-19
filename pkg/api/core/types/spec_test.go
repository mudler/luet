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

package types_test

import (
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"
	. "github.com/mudler/luet/pkg/api/core/types"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	. "github.com/mudler/luet/pkg/compiler"
	pkg "github.com/mudler/luet/pkg/database"
	"github.com/mudler/luet/pkg/tree"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Spec", func() {
	ginkgo.Context("Luet specs", func() {
		ginkgo.It("Allows normal operations", func() {
			testSpec := &LuetCompilationSpec{Package: &Package{Name: "foo", Category: "a", Version: "0"}}
			testSpec2 := &LuetCompilationSpec{Package: &Package{Name: "bar", Category: "a", Version: "0"}}
			testSpec3 := &LuetCompilationSpec{Package: &Package{Name: "baz", Category: "a", Version: "0"}}
			testSpec4 := &LuetCompilationSpec{Package: &Package{Name: "foo", Category: "a", Version: "0"}}

			specs := NewLuetCompilationspecs(testSpec, testSpec2)
			Expect(specs.Len()).To(Equal(2))
			Expect(specs.All()).To(Equal([]*LuetCompilationSpec{testSpec, testSpec2}))
			specs.Add(testSpec3)
			Expect(specs.All()).To(Equal([]*LuetCompilationSpec{testSpec, testSpec2, testSpec3}))
			specs.Add(testSpec4)
			Expect(specs.All()).To(Equal([]*LuetCompilationSpec{testSpec, testSpec2, testSpec3, testSpec4}))
			newSpec := specs.Unique()
			Expect(newSpec.All()).To(Equal([]*LuetCompilationSpec{testSpec, testSpec2, testSpec3}))

			newSpec2 := specs.Remove(NewLuetCompilationspecs(testSpec, testSpec2))
			Expect(newSpec2.All()).To(Equal([]*LuetCompilationSpec{testSpec3}))

		})
		ginkgo.Context("virtuals", func() {
			ginkgo.When("is empty", func() {
				ginkgo.It("is virtual", func() {
					spec := &LuetCompilationSpec{}
					Expect(spec.IsVirtual()).To(BeTrue())
				})
			})
			ginkgo.When("has defined steps", func() {
				ginkgo.It("is not a virtual", func() {
					spec := &LuetCompilationSpec{Steps: []string{"foo"}}
					Expect(spec.IsVirtual()).To(BeFalse())
				})
			})
			ginkgo.When("has defined image", func() {
				ginkgo.It("is not a virtual", func() {
					spec := &LuetCompilationSpec{Image: "foo"}
					Expect(spec.IsVirtual()).To(BeFalse())
				})
			})
		})
	})

	ginkgo.Context("Image hashing", func() {
		ginkgo.It("is stable", func() {
			spec1 := &LuetCompilationSpec{
				Image:        "foo",
				BuildOptions: &types.CompilerOptions{BuildValues: []map[string]interface{}{{"foo": "bar", "baz": true}}},

				Package: &Package{
					Name:     "foo",
					Category: "Bar",
					Labels: map[string]string{
						"foo": "bar",
						"baz": "foo",
					},
				},
			}
			spec2 := &LuetCompilationSpec{
				Image:        "foo",
				BuildOptions: &types.CompilerOptions{BuildValues: []map[string]interface{}{{"foo": "bar", "baz": true}}},
				Package: &Package{
					Name:     "foo",
					Category: "Bar",
					Labels: map[string]string{
						"foo": "bar",
						"baz": "foo",
					},
				},
			}
			spec3 := &LuetCompilationSpec{
				Image: "foo",
				Steps: []string{"foo"},
				Package: &Package{
					Name:     "foo",
					Category: "Bar",
					Labels: map[string]string{
						"foo": "bar",
						"baz": "foo",
					},
				},
			}
			hash, err := spec1.Hash()
			Expect(err).ToNot(HaveOccurred())

			hash2, err := spec2.Hash()
			Expect(err).ToNot(HaveOccurred())

			hash3, err := spec3.Hash()
			Expect(err).ToNot(HaveOccurred())

			Expect(hash).To(Equal(hash2))
			hashagain, err := spec2.Hash()
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).ToNot(Equal(hash3))
			Expect(hash).To(Equal(hashagain))
		})
	})

	ginkgo.Context("Simple package build definition", func() {
		ginkgo.It("Loads it correctly", func() {
			generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(nil, generalRecipe.GetDatabase())
			lspec, err := compiler.FromPackage(&Package{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
			Expect(lspec.Image).To(Equal("luet/base"))
			Expect(lspec.Seed).To(Equal("alpine"))
			tmpdir, err := os.MkdirTemp("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			lspec.Env = []string{"test=1"}
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
ENV PACKAGE_CATEGORY=app-admin
ENV test=1`))

			err = lspec.WriteStepImageDefinition(lspec.Image, filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err = fileHelper.Read(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM luet/base
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=enman
ENV PACKAGE_VERSION=1.4.0
ENV PACKAGE_CATEGORY=app-admin
ENV test=1
RUN echo foo > /test
RUN echo bar > /test2`))

		})

	})

	ginkgo.It("Renders retrieve and env fields", func() {
		generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

		err := generalRecipe.Load("../../../../tests/fixtures/retrieve")
		Expect(err).ToNot(HaveOccurred())

		Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

		compiler := NewLuetCompiler(nil, generalRecipe.GetDatabase())
		lspec, err := compiler.FromPackage(&Package{Name: "a", Category: "test", Version: "1.0"})
		Expect(err).ToNot(HaveOccurred())

		Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
		Expect(lspec.Image).To(Equal("luet/base"))
		Expect(lspec.Seed).To(Equal("alpine"))
		tmpdir, err := os.MkdirTemp("", "tree")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpdir) // clean up

		err = lspec.WriteBuildImageDefinition(filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		dockerfile, err := fileHelper.Read(filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		Expect(dockerfile).To(Equal(`
FROM alpine
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=a
ENV PACKAGE_VERSION=1.0
ENV PACKAGE_CATEGORY=test
ADD test /luetbuild/
ADD http://www.google.com /luetbuild/
ENV test=1`))

		lspec.SetOutputPath("/foo/bar")

		err = lspec.WriteBuildImageDefinition(filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		dockerfile, err = fileHelper.Read(filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		Expect(dockerfile).To(Equal(`
FROM alpine
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=a
ENV PACKAGE_VERSION=1.0
ENV PACKAGE_CATEGORY=test
ADD test /luetbuild/
ADD http://www.google.com /luetbuild/
ENV test=1`))

		err = lspec.WriteStepImageDefinition(lspec.Image, filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		dockerfile, err = fileHelper.Read(filepath.Join(tmpdir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())

		Expect(dockerfile).To(Equal(`
FROM luet/base
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=a
ENV PACKAGE_VERSION=1.0
ENV PACKAGE_CATEGORY=test
ADD test /luetbuild/
ADD http://www.google.com /luetbuild/
ENV test=1
RUN echo foo > /test
RUN echo bar > /test2`))

	})
})
