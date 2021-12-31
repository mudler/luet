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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	helpers "github.com/mudler/luet/tests/helpers"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	. "github.com/mudler/luet/pkg/compiler"
	sd "github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/compiler/types/compression"
	"github.com/mudler/luet/pkg/compiler/types/options"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Compiler", func() {
	ctx := context.NewContext()

	Context("Simple package build definition", func() {
		It("Compiles it correctly", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(2), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "chmod +x generate.sh", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))

			spec.SetOutputPath(tmpdir)

			artifact, err := compiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
			Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())

			content1, err := fileHelper.Read(spec.Rel("test5"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := fileHelper.Read(spec.Rel("test6"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("artifact5\n"))
			Expect(content2).To(Equal("artifact6\n"))

		})
	})

	Context("Copy and Join", func() {
		It("Compiles it correctly with Copy", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/copy")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(2), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.2"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifact, err := compiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
			Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("result"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("bina/busybox"))).To(BeTrue())
		})

		It("Compiles it correctly with Join", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/join")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(2), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.2"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifact, err := compiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
			Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec.Rel("newc"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test4"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test3"))).To(BeTrue())
		})
	})

	Context("Simple package build definition", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(1), options.WithContext(context.NewContext()))

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
			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
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
			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.WithContext(context.NewContext()))

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

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(2), options.WithContext(context.NewContext()))

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

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec, spec2, spec3))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(3))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}

			Expect(fileHelper.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test4"))).To(BeTrue())

			content1, err := fileHelper.Read(spec.Rel("c"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := fileHelper.Read(spec.Rel("cd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("c\n"))
			Expect(content2).To(Equal("c\n"))

			content1, err = fileHelper.Read(spec.Rel("d"))
			Expect(err).ToNot(HaveOccurred())
			content2, err = fileHelper.Read(spec.Rel("dd"))
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

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(1), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "base", Category: "layer", Version: "0.2"})
			Expect(err).ToNot(HaveOccurred())
			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			artifacts2, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec2))
			Expect(errs).To(BeNil())
			Expect(len(artifacts2)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}

			for _, artifact := range artifacts2 {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}

			Expect(fileHelper.Exists(spec.Rel("etc/hosts"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test1"))).To(BeTrue())
		})

		It("Compiles and includes ony wanted files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/include")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).ToNot(BeTrue())
		})

		It("Compiles and excludes files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludes")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvot"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles includes and excludes files", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludesincludes")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvot"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).ToNot(BeTrue())
		})

		It("Compiles and excludes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("marvin"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles includes and excludes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/excludeincludeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("marvin"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())
		})

		It("Compiles and includes ony wanted files also from unpacked packages", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/includeimage")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)
			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(fileHelper.Exists(spec.Rel("var/lib/udhcpd"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("marvin"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test5"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test2"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("lib/firmware"))).ToNot(BeTrue())
		})

		It("Compiles a more complex tree", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "pkgs-checker", Category: "package", Version: "9999"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(artifact.NewPackageArtifact(spec.Rel("extra-layer-0.1.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("extra-layer"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("usr/bin/pkgs-checker"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("base-layer-0.1.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("base-layer-0.1.metadata.yaml"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("extra-layer-0.1.metadata.yaml"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("extra-layer-0.1.package.tar"))).To(BeTrue())
		})

		It("Compiles with provides support", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/provides")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "d", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(artifact.NewPackageArtifact(spec.Rel("c-test-1.0.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("d"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("dd"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("c"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("cd"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("d-test-1.0.metadata.yaml"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("c-test-1.0.metadata.yaml"))).To(BeTrue())
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

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "d", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(1))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(artifact.NewPackageArtifact(spec.Rel("c-test-1.0.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("d"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("dd"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("c"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("cd"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("d-test-1.0.metadata.yaml"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("c-test-1.0.metadata.yaml"))).To(BeTrue())
		})
		It("Compiles revdeps", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "revdep")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/layered")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "extra", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(2))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(artifact.NewPackageArtifact(spec.Rel("extra-layer-0.1.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(spec.Rel("extra-layer"))).To(BeTrue())

			Expect(fileHelper.Exists(spec.Rel("usr/bin/pkgs-checker"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("base-layer-0.1.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("extra-layer-0.1.package.tar"))).To(BeTrue())
		})

		It("Generates a correct buildtree", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/complex/selection")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(10))
			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "vhba", Category: "sys-fs-5.4.2", Version: "20190410"})
			Expect(err).ToNot(HaveOccurred())

			bt, err := compiler.BuildTree(compilerspec.LuetCompilationspecs{*spec})
			Expect(err).ToNot(HaveOccurred())

			Expect(bt.AllLevels()).To(Equal([]int{0, 1, 2, 3, 4, 5}))
			Expect(bt.AllInLevel(0)).To(Equal([]string{"layer/build"}))
			Expect(bt.AllInLevel(1)).To(Equal([]string{"layer/sabayon-build-portage"}))
			Expect(bt.AllInLevel(2)).To(Equal([]string{"layer/build-sabayon-overlay"}))
			Expect(bt.AllInLevel(3)).To(Equal([]string{"layer/build-sabayon-overlays"}))
			Expect(bt.AllInLevel(4)).To(ContainElements("sys-kernel/linux-sabayon", "sys-kernel/sabayon-sources"))
			Expect(bt.AllInLevel(5)).To(Equal([]string{"sys-fs-5.4.2/vhba"}))
		})

		It("Compiles complex dependencies trees with best matches", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "complex")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/complex/selection")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(10))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "vhba", Category: "sys-fs-5.4.2", Version: "20190410"})
			Expect(err).ToNot(HaveOccurred())

			//		err = generalRecipe.Tree().ResolveDeps(3)
			//		Expect(err).ToNot(HaveOccurred())

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(6))
			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}
			Expect(artifact.NewPackageArtifact(spec.Rel("vhba-sys-fs-5.4.2-20190410.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec.Rel("sabayon-build-portage-layer-0.20191126.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("build-layer-0.1.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("build-sabayon-overlay-layer-0.20191212.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("build-sabayon-overlays-layer-0.1.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("linux-sabayon-sys-kernel-5.4.2.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("sabayon-sources-sys-kernel-5.4.2.package.tar"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("vhba"))).To(BeTrue())
		})

		It("Compiles revdeps with seeds", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			tmpdir, err := ioutil.TempDir("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/buildableseed")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(4))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileWithReverseDeps(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(4))

			for _, artifact := range artifacts {
				Expect(fileHelper.Exists(artifact.Path)).To(BeTrue())
				Expect(artifact.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			}

			// A deps on B, so A artifacts are here:
			Expect(fileHelper.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test4"))).To(BeTrue())

			// B
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("artifact42"))).To(BeTrue())

			// C depends on B, so B is here
			content1, err := fileHelper.Read(spec.Rel("c"))
			Expect(err).ToNot(HaveOccurred())
			content2, err := fileHelper.Read(spec.Rel("cd"))
			Expect(err).ToNot(HaveOccurred())
			Expect(content1).To(Equal("c\n"))
			Expect(content2).To(Equal("c\n"))

			// D is here as it requires C, and C was recompiled
			content1, err = fileHelper.Read(spec.Rel("d"))
			Expect(err).ToNot(HaveOccurred())
			content2, err = fileHelper.Read(spec.Rel("dd"))
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

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.Concurrency(2), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			for _, a := range artifacts {
				Expect(fileHelper.Exists(a.Path)).To(BeTrue())
				Expect(a.Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())

				for _, d := range a.Dependencies {
					Expect(fileHelper.Exists(d.Path)).To(BeTrue())
					Expect(artifact.NewPackageArtifact(d.Path).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
				}
			}

			Expect(fileHelper.Exists(spec.Rel("test3"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test4"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test6"))).To(BeTrue())

		})
	})

	Context("Packages which conents are the container image", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(1))

			Expect(artifact.NewPackageArtifact(filepath.Join(tmpdir, "runtime-layer-0.1.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec.Rel("bin/busybox"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("var"))).ToNot(BeTrue())
		})

		It("Pushes final images along", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			randString := strings.ToLower(helpers.String(10))
			imageName := fmt.Sprintf("ttl.sh/%s", randString)
			b := sd.NewSimpleDockerBackend(ctx)

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(b, generalRecipe.GetDatabase(),
				options.EnablePushFinalImages, options.ForcePushFinalImages, options.WithFinalRepository(imageName))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "build", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(2))
			//Expect(len(artifacts[0].Dependencies)).To(Equal(1))

			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", imageName, artifacts[0].Runtime.ImageID()))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", imageName, artifacts[0].Runtime.GetMetadataFilePath()))).To(BeTrue())

			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", imageName, artifacts[1].Runtime.ImageID()))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", imageName, artifacts[1].Runtime.GetMetadataFilePath()))).To(BeTrue())

			img, err := b.ImageReference(fmt.Sprintf("%s:%s", imageName, artifacts[0].Runtime.ImageID()), true)
			Expect(err).ToNot(HaveOccurred())
			_, path, err := image.Extract(ctx, img, nil)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(path) // clean up

			Expect(fileHelper.Exists(filepath.Join(path, "bin/busybox"))).To(BeTrue())

			img, err = b.ImageReference(fmt.Sprintf("%s:%s", imageName, artifacts[1].Runtime.GetMetadataFilePath()), true)
			Expect(err).ToNot(HaveOccurred())
			_, path, err = image.Extract(ctx, img, nil)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(path) // clean up

			meta := filepath.Join(path, artifacts[1].Runtime.GetMetadataFilePath())
			Expect(fileHelper.Exists(meta)).To(BeTrue())

			d, err := ioutil.ReadFile(meta)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(d)).To(ContainSubstring(artifacts[1].CompileSpec.GetPackage().GetName()))
		})
	})

	Context("Packages which conents are a package folder", func() {
		It("Compiles it in parallel", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/package_dir")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase())

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

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec, spec2))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(2))
			Expect(len(artifacts[0].Dependencies)).To(Equal(0))

			Expect(artifact.NewPackageArtifact(filepath.Join(tmpdir, "dironly-test-1.0.package.tar")).Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec.Rel("test1"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("test2"))).To(BeTrue())

			Expect(artifact.NewPackageArtifact(filepath.Join(tmpdir2, "dironly_filter-test-1.0.package.tar")).Unpack(ctx, tmpdir2, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec2.Rel("test5"))).To(BeTrue())
			Expect(fileHelper.Exists(spec2.Rel("test6"))).ToNot(BeTrue())
			Expect(fileHelper.Exists(spec2.Rel("artifact42"))).ToNot(BeTrue())
		})
	})

	Context("Compression", func() {
		It("Builds packages in gzip", func() {
			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/packagelayers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(2))

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			compiler.Options.CompressionType = compression.GZip
			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(1))
			Expect(fileHelper.Exists(spec.Rel("runtime-layer-0.1.package.tar.gz"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("runtime-layer-0.1.package.tar"))).To(BeFalse())
			Expect(artifacts[0].Unpack(ctx, tmpdir, false)).ToNot(HaveOccurred())
			//	Expect(helpers.Untar(spec.Rel("runtime-layer-0.1.package.tar"), tmpdir, false)).ToNot(HaveOccurred())
			Expect(fileHelper.Exists(spec.Rel("bin/busybox"))).To(BeTrue())
			Expect(fileHelper.Exists(spec.Rel("var"))).ToNot(BeTrue())
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
			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.WithContext(context.NewContext()))

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

			compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), options.WithContext(context.NewContext()))

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "runtime", Category: "layer", Version: "0.1"})
			Expect(err).ToNot(HaveOccurred())
			compiler.Options.CompressionType = compression.GZip
			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)

			artifacts, errs := compiler.CompileParallel(false, compilerspec.NewLuetCompilationspecs(spec))
			Expect(errs).To(BeNil())
			Expect(len(artifacts)).To(Equal(1))
			Expect(len(artifacts[0].Dependencies)).To(Equal(1))
			Expect(artifacts[0].Files).To(ContainElement("bin/busybox"))

			Expect(fileHelper.Exists(spec.Rel("runtime-layer-0.1.metadata.yaml"))).To(BeTrue())

			art, err := LoadArtifactFromYaml(spec)
			Expect(err).ToNot(HaveOccurred())

			files := art.Files
			Expect(files).To(ContainElement("bin/busybox"))
		})
	})
})
