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

	. "github.com/mudler/luet/pkg/compiler"
	helpers "github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Spec", func() {
	Context("Luet specs", func() {
		It("Allows normal operations", func() {
			testSpec := &LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "foo", Category: "a", Version: "0"}}
			testSpec2 := &LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "bar", Category: "a", Version: "0"}}
			testSpec3 := &LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "baz", Category: "a", Version: "0"}}
			testSpec4 := &LuetCompilationSpec{Package: &pkg.DefaultPackage{Name: "foo", Category: "a", Version: "0"}}

			specs := NewLuetCompilationspecs(testSpec, testSpec2)
			Expect(specs.Len()).To(Equal(2))
			Expect(specs.All()).To(Equal([]CompilationSpec{testSpec, testSpec2}))
			specs.Add(testSpec3)
			Expect(specs.All()).To(Equal([]CompilationSpec{testSpec, testSpec2, testSpec3}))
			specs.Add(testSpec4)
			Expect(specs.All()).To(Equal([]CompilationSpec{testSpec, testSpec2, testSpec3, testSpec4}))
			newSpec := specs.Unique()
			Expect(newSpec.All()).To(Equal([]CompilationSpec{testSpec, testSpec2, testSpec3}))

			newSpec2 := specs.Remove(NewLuetCompilationspecs(testSpec, testSpec2))
			Expect(newSpec2.All()).To(Equal([]CompilationSpec{testSpec3}))

		})
	})

	Context("Simple package build definition", func() {
		It("Loads it correctly", func() {
			generalRecipe := tree.NewGeneralRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(nil, generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			lspec, ok := spec.(*LuetCompilationSpec)
			Expect(ok).To(BeTrue())

			Expect(lspec.Steps).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))
			Expect(lspec.Image).To(Equal("luet/base"))
			Expect(lspec.Seed).To(Equal("alpine"))
			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = lspec.WriteBuildImageDefinition(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err := helpers.Read(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM alpine
COPY . /luetbuild
WORKDIR /luetbuild
`))

			err = lspec.WriteStepImageDefinition(lspec.Image, filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			dockerfile, err = helpers.Read(filepath.Join(tmpdir, "Dockerfile"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dockerfile).To(Equal(`
FROM luet/base
RUN echo foo > /test
RUN echo bar > /test2`))

		})

	})
})
