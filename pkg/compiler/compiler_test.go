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

	. "github.com/mudler/luet/pkg/compiler"
	sd "github.com/mudler/luet/pkg/compiler/backend"
	helpers "github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Compiler", func() {
	Context("Simple package build definition", func() {
		It("Compiles it correctly", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "chmod +x generate.sh", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(2)

			artifact, err := compiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
			Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())

			content1, err := helpers.Read(spec.Rel("test5"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := helpers.Read(spec.Rel("test6"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("artifact5\n"))
			Expect(content2).To(Equal("artifact6\n"))

		})
	})

	Context("Simple package build definition", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			compiler.SetConcurrency(2)
			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

		})
	})

	Context("Templated packages", func() {
		It("Renders", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/templates")
			Expect(err).ToNot(HaveOccurred())
			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))
			pkg, err := generalRecipe.GetDatabase().FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(pkg)
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.GetImage()).To(Equal("b:bar"))
		})
	})

	Context("Reconstruct image tree", func() {
		It("Compiles it", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/buildableseed")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(4))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec3, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "d", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			Expect(spec3.GetPackage().GetRequires()[0].GetName()).To(Equal("c"))

			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			spec3.SetOutputPath(tmpdir)
			compiler.SetConcurrency(2)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec, spec2, spec3))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(3))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

			Expect(helpers.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test4"))).To(BeTrue())

			content1, err := helpers.Read(spec.Rel("c"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := helpers.Read(spec.Rel("cd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("c\n"))
			Expect(content2).To(Equal("c\n"))

			content1, err = helpers.Read(spec.Rel("d"))
			Expect(err).ToNot(HaveOccurred())
			content2, err = helpers.Read(spec.Rel("dd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("s\n"))
			Expect(content2).To(Equal("dd\n"))
		})

		It("unpacks images when needed", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "base", Category: "layer", Version: "0.2"})
			Expect(err).ToNot(HaveOccurred())
			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)
			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			artifacts2, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec2))
			Expect(errs).To(BeNil())
			Expect(len(artifacts2)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

			for _, artifact := range artifacts2 {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

			Expect(helpers.Exists(spec.Rel("etc/hosts"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test1"))).To(BeTrue())
		})

		It("Compiles and includes ony wanted files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/include")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).ToNot(BeTrue())
		})

		It("Compiles and excludes files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludes")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvot"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles includes and excludes files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludesincludes")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvot"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).ToNot(BeTrue())
		})

		It("Compiles and excludes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)
			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("marvin"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles includes and excludes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludeincludeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)
			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("marvin"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles and includes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/includeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)
			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Exists(spec.Rel("var/lib/udhcpd"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test5"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("test2"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("lib/firmware"))).ToNot(BeTrue())
		})

		It("Compiles a more complex tree", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "pkgs-checker", Category: "package", Version: "9999"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Untar(spec.Rel("extra-layer-0.1.package.tar"), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("extra-layer"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("usr/bin/pkgs-checker"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("base-layer-0.1.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("base-layer-0.1.metadata.yaml"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("extra-layer-0.1.metadata.yaml"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("extra-layer-0.1.package.tar"))).To(BeTrue())
		})

		It("Compiles with provides support", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/provides")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "d", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Untar(spec.Rel("c-test-1.0.package.tar"), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("d"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("dd"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("c"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("cd"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("d-test-1.0.metadata.yaml"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("c-test-1.0.metadata.yaml"))).To(BeTrue())
		})

		It("Compiles with provides and selectors support", func() {
			// Same test as before, but fixtures differs
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/provides_selector")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "d", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Untar(spec.Rel("c-test-1.0.package.tar"), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("d"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("dd"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("c"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("cd"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("d-test-1.0.metadata.yaml"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("c-test-1.0.metadata.yaml"))).To(BeTrue())
		})
		It("Compiles revdeps", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "revdep")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(2))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Untar(spec.Rel("extra-layer-0.1.package.tar"), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("extra-layer"))).To(BeTrue())

			Expect(helpers.Exists(spec.Rel("usr/bin/pkgs-checker"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("base-layer-0.1.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("extra-layer-0.1.package.tar"))).To(BeTrue())
		})

		It("Compiles complex dependencies trees with best matches", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "complex")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/complex/selection")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(10))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "vhba", Category: "sys-fs-5.4.2", Version: "20190410"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(6))
			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(helpers.Untar(spec.Rel("vhba-sys-fs-5.4.2-20190410.package.tar"), tmpdir, false)).ToNot(HaveOccurred())
			Expect(helpers.Exists(spec.Rel("sabayon-build-portage-layer-0.20191126.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("build-layer-0.1.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("build-sabayon-overlay-layer-0.20191212.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("build-sabayon-overlays-layer-0.1.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("linux-sabayon-sys-kernel-5.4.2.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("sabayon-sources-sys-kernel-5.4.2.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("vhba"))).To(BeTrue())
		})

		It("Compiles revdeps with seeds", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/buildableseed")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(4))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(4))

			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

			// A deps on B, so A artifacts are here:
			Expect(helpers.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test4"))).To(BeTrue())

			// B
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("artifact42"))).To(BeTrue())

			// C depends on B, so B is here
			content1, err := helpers.Read(spec.Rel("c"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := helpers.Read(spec.Rel("cd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("c\n"))
			Expect(content2).To(Equal("c\n"))

			// D is here as it requires C, and C was recompiled
			content1, err = helpers.Read(spec.Rel("d"))
			Expect(err).ToNot(HaveOccurred())
			content2, err = helpers.Read(spec.Rel("dd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("s\n"))
			Expect(content2).To(Equal("dd\n"))
		})

	})

	Context("Simple package build definition", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/expansion")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(2)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

				for _, d := range artifact.GetDependencies() {
					Expect(helpers.Exists(d.GetPath())).To(BeTrue())
					Expect(helpers.Untar(d.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
				}
			}

			Expect(helpers.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test4"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())

		})
	})

	Context("Packages which conents are the container image", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(1))
			Expect(helpers.Untar(spec.Rel("runtime-layer-0.1.package.tar"), tmpdir, false)).ToNot(HaveOccurred())
			Expect(helpers.Exists(spec.Rel("bin/busybox"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("var"))).ToNot(BeTrue())
		})
	})

	Context("Packages which conents are a package folder", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/package_dir")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{
				Name:     "dironly",
				Category: "test",
				Version:  "1.0",
			})
			Expect(err).ToNot(HaveOccurred())

			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{
				Name:     "dironly_filter",
				Category: "test",
				Version:  "1.0",
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up
			tmpdir2, err := ioutil.TempDir("", "tree2")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir2) // clean up

			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir2)

			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(2))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(0))

			Expect(helpers.Untar(spec.Rel("dironly-test-1.0.package.tar"), tmpdir, false)).ToNot(HaveOccurred())
			Expect(helpers.Exists(spec.Rel("test1"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test2"))).To(BeTrue())

			Expect(helpers.Untar(spec2.Rel("dironly_filter-test-1.0.package.tar"), tmpdir2, false)).ToNot(HaveOccurred())
			Expect(helpers.Exists(spec2.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec2.Rel("test6"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec2.Rel("artifact42"))).ToNot(BeTrue())
		})
	})

	Context("Compression", func() {
		It("Builds packages in gzip", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			compiler.SetCompressionType(GZip)
			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(1))
			Expect(helpers.Exists(spec.Rel("runtime-layer-0.1.package.tar.gz"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("runtime-layer-0.1.package.tar"))).To(BeFalse())
			Expect(artifacts[0].Unpack(tmpdir, false)).ToNot(HaveOccurred())
			//	Expect(helpers.Untar(spec.Rel("runtime-layer-0.1.package.tar"), tmpdir, false)).ToNot(HaveOccurred())
			Expect(helpers.Exists(spec.Rel("bin/busybox"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("var"))).ToNot(BeTrue())
		})
	})

	Context("Compilation of whole tree", func() {
		It("doesn't include dependencies that would be compiled anyway", func() {
			// As some specs are dependent from each other, don't pull it in if they would
			// be eventually
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/includeimage")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))
			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			specs, err := compiler.FromDatabase(generalRecipe.GetDatabase(), true, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(specs)).To(Equal(1))

			Expect(specs[0].GetPackage().GetFingerPrint()).To(Equal("b-test-1.0"))
		})
	})

	Context("File list", func() {
		It("is generated after the compilation process and annotated in the metadata", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			compiler.SetCompressionType(GZip)
			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

			artifacts, errs := compiler.CompileParallel(false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].GetDependencies())).To(Equal(1))
			Expect(artifacts[0].GetFiles()).To(ContainElement("bin/busybox"))

			Expect(helpers.Exists(spec.Rel("runtime-layer-0.1.metadata.yaml"))).To(BeTrue())

			art, err := LoadArtifactFromYaml(spec)
			Expect(err).ToNot(HaveOccurred())

			files := art.GetFiles()
			Expect(files).To(ContainElement("bin/busybox"))
		})
	})
})
