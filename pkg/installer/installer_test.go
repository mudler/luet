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

package installer_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	//	. "github.com/mudler/luet/pkg/installer"
	compiler "github.com/mudler/luet/pkg/compiler"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/installer"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installer", func() {
	Context("Writes a repository definition", func() {
		It("Writes a repo and can install packages from it", func() {
			//repo:=NewLuetRepository()

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2", "chmod +x generate.sh"}))

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

			Expect(helpers.Exists(spec.Rel("b-test-1.0.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("b-test-1.0.metadata.yaml"))).To(BeTrue())

			repo, err := GenerateRepository("test", tmpdir, "local", 1, tmpdir, "../../tests/fixtures/buildable", pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel("repository.yaml"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("repository.yaml"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).To(BeTrue())
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))

			fakeroot, err := ioutil.TempDir("", "fakeroot")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(fakeroot) // clean up

			inst := NewLuetInstaller(1)
			repo2, err := NewLuetRepositoryFromYaml([]byte(`
name: "test"
type: "local"
uri: "`+tmpdir+`"
`), pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())

			inst.Repositories(Repositories{repo2})
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))
			systemDB := pkg.NewInMemoryDatabase(false)
			system := &System{Database: systemDB, Target: fakeroot}
			err = inst.Install([]pkg.Package{&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}}, system)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).To(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).To(BeTrue())
			_, err = systemDB.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			files, err := systemDB.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(files).To(Equal([]string{"artifact42", "test5", "test6"}))
			Expect(err).ToNot(HaveOccurred())

			Expect(len(system.Database.GetPackages())).To(Equal(1))
			p, err := system.Database.GetPackage(system.Database.GetPackages()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetName()).To(Equal("b"))

			err = inst.Uninstall(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}, system)
			Expect(err).ToNot(HaveOccurred())

			// Nothing should be there anymore (files, packagedb entry)
			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).ToNot(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).ToNot(BeTrue())

			_, err = systemDB.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())
			_, err = systemDB.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())
		})

	})

	Context("Installation", func() {
		It("Installs in a system with a persistent db", func() {
			//repo:=NewLuetRepository()

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase())

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2", "chmod +x generate.sh"}))

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

			Expect(helpers.Exists(spec.Rel("b-test-1.0.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("b-test-1.0.metadata.yaml"))).To(BeTrue())

			repo, err := GenerateRepository("test", tmpdir, "local", 1, tmpdir, "../../tests/fixtures/buildable", pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel("repository.yaml"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("repository.yaml"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).To(BeTrue())
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))

			fakeroot, err := ioutil.TempDir("", "fakeroot")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(fakeroot) // clean up

			inst := NewLuetInstaller(1)
			repo2, err := NewLuetRepositoryFromYaml([]byte(`
name: "test"
type: "local"
uri: "`+tmpdir+`"
`), pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())

			inst.Repositories(Repositories{repo2})
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))

			bolt, err := ioutil.TempDir("", "db")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(bolt) // clean up

			systemDB := pkg.NewBoltDatabase(filepath.Join(bolt, "db.db"))
			system := &System{Database: systemDB, Target: fakeroot}
			err = inst.Install([]pkg.Package{&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}}, system)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).To(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).To(BeTrue())
			_, err = systemDB.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(system.Database.GetPackages())).To(Equal(1))
			p, err := system.Database.GetPackage(system.Database.GetPackages()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetName()).To(Equal("b"))

			files, err := systemDB.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(files).To(Equal([]string{"artifact42", "test5", "test6"}))
			Expect(err).ToNot(HaveOccurred())

			err = inst.Uninstall(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}, system)
			Expect(err).ToNot(HaveOccurred())

			// Nothing should be there anymore (files, packagedb entry)
			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).ToNot(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).ToNot(BeTrue())

			_, err = system.Database.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())
			_, err = system.Database.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())

		})

	})

	Context("Simple upgrades", func() {
		It("Installs packages and Upgrades a system with a persistent db", func() {
			//repo:=NewLuetRepository()

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/upgrade")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(4))

			c := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase())

			spec, err := c.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())
			spec2, err := c.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.1"})
			Expect(err).ToNot(HaveOccurred())
			spec3, err := c.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			spec3.SetOutputPath(tmpdir)
			c.SetConcurrency(2)

			_, errs := c.CompileParallel(false, compiler.NewLuetCompilationspecs(spec, spec2, spec3))

			Expect(errs).To(BeEmpty())

			repo, err := GenerateRepository("test", tmpdir, "local", 1, tmpdir, "../../tests/fixtures/upgrade", pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel("repository.yaml"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("repository.yaml"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("tree.tar"))).To(BeTrue())
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))

			fakeroot, err := ioutil.TempDir("", "fakeroot")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(fakeroot) // clean up

			inst := NewLuetInstaller(1)
			repo2, err := NewLuetRepositoryFromYaml([]byte(`
name: "test"
type: "local"
uri: "`+tmpdir+`"
`), pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())

			inst.Repositories(Repositories{repo2})
			Expect(repo.GetUri()).To(Equal(tmpdir))
			Expect(repo.GetType()).To(Equal("local"))

			bolt, err := ioutil.TempDir("", "db")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(bolt) // clean up

			systemDB := pkg.NewBoltDatabase(filepath.Join(bolt, "db.db"))
			system := &System{Database: systemDB, Target: fakeroot}
			err = inst.Install([]pkg.Package{&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}}, system)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).To(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).To(BeTrue())
			_, err = systemDB.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(system.Database.GetPackages())).To(Equal(1))
			p, err := system.Database.GetPackage(system.Database.GetPackages()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(p.GetName()).To(Equal("b"))

			files, err := systemDB.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(files).To(Equal([]string{"artifact42", "test5", "test6"}))
			Expect(err).ToNot(HaveOccurred())

			err = inst.Upgrade(system)
			Expect(err).ToNot(HaveOccurred())

			// Nothing should be there anymore (files, packagedb entry)
			Expect(helpers.Exists(filepath.Join(fakeroot, "test5"))).ToNot(BeTrue())
			Expect(helpers.Exists(filepath.Join(fakeroot, "test6"))).ToNot(BeTrue())

			// New version - new files
			Expect(helpers.Exists(filepath.Join(fakeroot, "newc"))).To(BeTrue())
			_, err = system.Database.GetPackageFiles(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())
			_, err = system.Database.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).To(HaveOccurred())

			// New package should be there
			_, err = system.Database.FindPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.1"})
			Expect(err).ToNot(HaveOccurred())

		})

	})

})
