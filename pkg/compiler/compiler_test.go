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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2", "chmod +x generate.sh"}))

			spec.SetOutputPath(tmpdir)
			artifact, err := compiler.Compile(2, false, spec)
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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

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
			artifacts, errs := compiler.CompileParallel(2, false, NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			for _, artifact := range artifacts {
				Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
				Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())
			}

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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(4))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())

			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

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

			artifacts, errs := compiler.CompileParallel(2, false, NewLuetCompilationspecs(spec, spec2, spec3))
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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "base", Category: "layer", Version: "0.2"})
			Expect(err).ToNot(HaveOccurred())
			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(1, false, NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			artifacts2, errs := compiler.CompileParallel(1, false, NewLuetCompilationspecs(spec2))
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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(1, false, NewLuetCompilationspecs(spec))
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

		It("Compiles a more complex tree", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "pkgs-checker", Category: "package", Version: "9999"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(1, false, NewLuetCompilationspecs(spec))
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
			Expect(helpers.Exists(spec.Rel("extra-layer-0.1.package.tar"))).To(BeTrue())
		})

		It("Compiles revdeps", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "revdep")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(1, false, NewLuetCompilationspecs(spec))
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

		It("Compiles revdeps with seeds", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/buildableseed")
			Expect(err).ToNot(HaveOccurred())
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(4))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(1, false, NewLuetCompilationspecs(spec))
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
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.Tree(), generalRecipe.Tree().GetPackageSet())
			err = compiler.Prepare(1)
			Expect(err).ToNot(HaveOccurred())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			artifacts, errs := compiler.CompileParallel(2, false, NewLuetCompilationspecs(spec))
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
})
