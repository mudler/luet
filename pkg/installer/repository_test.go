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

	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/compiler"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	config "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	"github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/installer"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func dockerStubRepo(tmpdir, tree, image string, push, force bool) (installer.Repository, error) {
	return GenerateRepository(
		"test",
		"description",
		"docker",
		[]string{image},
		1,
		tmpdir,
		[]string{tree},
		pkg.NewInMemoryDatabase(false), backend.NewSimpleDockerBackend(), image, push, force)
}

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

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "chmod +x generate.sh", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))

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

			repo, err := stubRepo(tmpdir, "../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir, false, true)
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

			Expect(spec.BuildSteps()).To(Equal([]string{"echo artifact5 > /test5", "echo artifact6 > /test6", "chmod +x generate.sh", "./generate.sh"}))
			Expect(spec.GetPreBuildSteps()).To(Equal([]string{"echo foo > /test", "echo bar > /test2"}))

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

			repo, err := stubRepo(tmpdir, "../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(tmpdir, false, true)
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
	Context("Docker repository", func() {
		repoImage := os.Getenv("UNIT_TEST_DOCKER_IMAGE_REPOSITORY")

		BeforeEach(func() {
			if repoImage == "" {
				Skip("UNIT_TEST_DOCKER_IMAGE_REPOSITORY not specified")
			}
		})

		It("generates images", func() {
			b := backend.NewSimpleDockerBackend()
			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(3))

			localcompiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), compiler.NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := localcompiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			localcompiler.SetConcurrency(1)

			artifact, err := localcompiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
			Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

			Expect(helpers.Exists(spec.Rel("test5"))).To(BeTrue())
			Expect(helpers.Exists(spec.Rel("test6"))).To(BeTrue())

			repo, err := dockerStubRepo(tmpdir, "../../tests/fixtures/buildable", repoImage, true, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(repoImage, false, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "tree.tar.gz"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "repository.meta.yaml.tar"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "repository.yaml"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "b-test-1.0"))).To(BeTrue())

			extracted, err := ioutil.TempDir("", "extracted")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(extracted) // clean up

			c := repo.Client()

			f, err := c.DownloadFile("repository.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Read(f)).To(ContainSubstring("name: test"))

			a, err := c.DownloadArtifact(&compiler.PackageArtifact{
				Path: "test.tar",
				CompileSpec: &compiler.LuetCompilationSpec{
					Package: &pkg.DefaultPackage{
						Name:     "b",
						Category: "test",
						Version:  "1.0",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(a.Unpack(extracted, false)).ToNot(HaveOccurred())
			Expect(helpers.Read(filepath.Join(extracted, "test6"))).To(Equal("artifact6\n"))
		})

		It("generates images of virtual packages", func() {
			b := backend.NewSimpleDockerBackend()
			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err = generalRecipe.Load("../../tests/fixtures/virtuals")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(5))

			localcompiler := compiler.NewLuetCompiler(backend.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), compiler.NewDefaultCompilerOptions(), solver.Options{Type: solver.SingleCoreSimple})

			spec, err := localcompiler.FromPackage(&pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.99"})
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.GetPackage().GetPath()).ToNot(Equal(""))

			tmpdir, err = ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			spec.SetOutputPath(tmpdir)
			localcompiler.SetConcurrency(1)

			artifact, err := localcompiler.Compile(false, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Exists(artifact.GetPath())).To(BeTrue())
			Expect(helpers.Untar(artifact.GetPath(), tmpdir, false)).ToNot(HaveOccurred())

			repo, err := dockerStubRepo(tmpdir, "../../tests/fixtures/virtuals", repoImage, true, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.GetName()).To(Equal("test"))
			Expect(helpers.Exists(spec.Rel(REPOSITORY_SPECFILE))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(TREE_TARBALL + ".gz"))).ToNot(BeTrue())
			Expect(helpers.Exists(spec.Rel(REPOSITORY_METAFILE + ".tar"))).ToNot(BeTrue())
			err = repo.Write(repoImage, false, true)
			Expect(err).ToNot(HaveOccurred())

			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "tree.tar.gz"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "repository.meta.yaml.tar"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "repository.yaml"))).To(BeTrue())
			Expect(b.ImageAvailable(fmt.Sprintf("%s:%s", repoImage, "a-test-1.99"))).To(BeTrue())

			extracted, err := ioutil.TempDir("", "extracted")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(extracted) // clean up

			c := repo.Client()

			f, err := c.DownloadFile("repository.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Read(f)).To(ContainSubstring("name: test"))

			a, err := c.DownloadArtifact(&compiler.PackageArtifact{
				Path: "test.tar",
				CompileSpec: &compiler.LuetCompilationSpec{
					Package: &pkg.DefaultPackage{
						Name:     "a",
						Category: "test",
						Version:  "1.99",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(a.Unpack(extracted, false)).ToNot(HaveOccurred())

			Expect(helpers.DirectoryIsEmpty(extracted)).To(BeFalse())
			content, err := ioutil.ReadFile(filepath.Join(extracted, ".virtual"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(content)).To(Equal(""))
		})

		It("Searches files", func() {
			repos := Repositories{
				&LuetSystemRepository{
					Index: compiler.ArtifactIndex{
						&compiler.PackageArtifact{
							CompileSpec: &compiler.LuetCompilationSpec{
								Package: &pkg.DefaultPackage{},
							},
							Path:  "bar",
							Files: []string{"boo"},
						},
						&compiler.PackageArtifact{
							Path:  "d",
							Files: []string{"baz"},
						},
					},
				},
			}

			matches := repos.SearchPackages("bo", FileSearch)
			Expect(len(matches)).To(Equal(1))
			Expect(matches[0].Artifact.GetPath()).To(Equal("bar"))
		})

		It("Searches packages", func() {
			repo := &LuetSystemRepository{
				Index: compiler.ArtifactIndex{
					&compiler.PackageArtifact{
						Path: "foo",
						CompileSpec: &compiler.LuetCompilationSpec{
							Package: &pkg.DefaultPackage{
								Name:     "foo",
								Category: "bar",
								Version:  "1.0",
							},
						},
					},
					&compiler.PackageArtifact{
						Path: "baz",
						CompileSpec: &compiler.LuetCompilationSpec{
							Package: &pkg.DefaultPackage{
								Name:     "foo",
								Category: "baz",
								Version:  "1.0",
							},
						},
					},
				},
			}

			a, err := repo.SearchArtefact(&pkg.DefaultPackage{
				Name:     "foo",
				Category: "baz",
				Version:  "1.0",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(a.GetPath()).To(Equal("baz"))

			a, err = repo.SearchArtefact(&pkg.DefaultPackage{
				Name:     "foo",
				Category: "bar",
				Version:  "1.0",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(a.GetPath()).To(Equal("foo"))

			// Doesn't exist. so must fail
			_, err = repo.SearchArtefact(&pkg.DefaultPackage{
				Name:     "foo",
				Category: "bar",
				Version:  "1.1",
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
