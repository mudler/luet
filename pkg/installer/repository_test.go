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

	//	. "github.com/mudler/luet/pkg/installer"

	"io/ioutil"
	"os"

	"github.com/mudler/luet/pkg/compiler"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	config "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/installer"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository", func() {
	Context("Generation", func() {
		It("Generate repository metadata", func() {

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), compiler.NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2", "chmod +x generate.sh"}))

			spec.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)

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

			repo, err := GenerateRepository("test", "description", "disk", []string{tmpdir}, 1, tmpdir, []string{"../../tests/fixtures/buildable"}, pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).To(BeTrue())
		})

		It("Generate repository metadata of files ONLY referenced in a tree", func() {

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			generalRecipe2 := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe2.Load("../../tests/fixtures/finalizers")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe2.GetDatabase().GetPackages())).To(Equal(1))
			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			compiler2 := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe2.GetDatabase(), compiler.NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})
			spec2, err := compiler2.FromPackage(&pkg.DefaultPackage{Name: "alpine", Category: "seed", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			compiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), compiler.NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))
			Expect(spec2.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2", "chmod +x generate.sh"}))

			spec.SetOutputPath(tmpdir)
			spec2.SetOutputPath(tmpdir)
			compiler.SetConcurrency(1)
			compiler2.SetConcurrency(1)

			artifact, err := compiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
			Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

			artifact2, err := compiler2.Compile(false, spec2)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(artifact2.GetPath())).To(BeTrue())
			Expect(helpers.Untar(artifact2.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

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
			Expect(helpers.Exists(spec2.Rel("alpine-seed-1.0.package.tar"))).To(BeTrue())
			Expect(helpers.Exists(spec2.Rel("alpine-seed-1.0.metadata.yaml"))).To(BeTrue())

			repo, err := GenerateRepository("test", "description", "disk", []string{tmpdir}, 1, tmpdir, []string{"../../tests/fixtures/buildable"}, pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).To(BeTrue())

			// We check now that the artifact not referenced in the tree
			// (spec2) is not indexed in the repository
			repository, err := NewLuetSystemRepositoryFromYaml([]byte(`
name: "test"
type: "disk"
urls:
  - "`+tmpdir+`"
`), pkg.NewInMemoryDatabase(false))
			Expect(err).ToNot(HaveOccurred())
			repos, err := repository.Sync(true)
			Expect(err).ToNot(HaveOccurred())

			_, err = repos.GetTree().GetDatabase().FindPackage(spec.GetPackage())
			Expect(err).ToNot(HaveOccurred())
			_, err = repos.GetTree().GetDatabase().FindPackage(spec2.GetPackage())
			Expect(err).To(HaveOccurred()) // should throw error
		})
	})
	Context("Matching packages", func() {
		It("Matches packages in different repositories by priority", func() {
			package1 := &pkg.DefaultPackage{Name: "Test"}
			package2 := &pkg.DefaultPackage{Name: "Test2"}
			builder1 := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
			builder2 := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))

			_, err := builder1.GetDatabase().CreatePackage(package1)
			Expect(err).ToNot(HaveOccurred())

			_, err = builder2.GetDatabase().CreatePackage(package2)
			Expect(err).ToNot(HaveOccurred())
			repo1 := &LuetSystemRepository{LuetRepository: &config.LuetRepository{Name: "test1"}, Tree: builder1}
			repo2 := &LuetSystemRepository{LuetRepository: &config.LuetRepository{Name: "test2"}, Tree: builder2}
			repositories := Repositories{repo1, repo2}
			matches := repositories.PackageMatches([]pkg.Package{package1})
			Expect(matches).To(Equal([]PackageMatch{{Repo: repo1, Package: package1}}))

		})

	})
})
