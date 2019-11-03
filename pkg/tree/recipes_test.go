// Recipe is a builder imeplementation.

// It reads a Tree and spit it in human readable form (YAML), called recipe,
// It also loads a tree (recipe) from a YAML (to a db, e.g. BoltDB), allowing to query it
// with the solver, using the package object.
package tree_test

import (
	"io/ioutil"
	"os"

	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	gentoo "github.com/mudler/luet/pkg/tree/builder/gentoo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/mudler/luet/pkg/tree"
)

type FakeParser struct {
}

var _ = Describe("Recipe", func() {
	for _, dbType := range []gentoo.MemoryDB{gentoo.InMemory, gentoo.BoltDB} {
		Context("Tree generation and storing", func() {
			It("parses and writes a tree", func() {
				tmpdir, err := ioutil.TempDir("", "tree")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				gb := gentoo.NewGentooBuilder(&gentoo.SimpleEbuildParser{}, 20, dbType)
				tree, err := gb.Generate("../../tests/fixtures/overlay")
				Expect(err).ToNot(HaveOccurred())
				defer func() {
					Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))

				generalRecipe := NewGeneralRecipe()
				generalRecipe.WithTree(tree)
				err = generalRecipe.Save(tmpdir)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Reloading trees", func() {
			It("writes and reads back the same tree", func() {
				tmpdir, err := ioutil.TempDir("", "tree")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				gb := gentoo.NewGentooBuilder(&gentoo.SimpleEbuildParser{}, 20, dbType)
				tree, err := gb.Generate("../../tests/fixtures/overlay")
				Expect(err).ToNot(HaveOccurred())
				defer func() {
					Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))

				generalRecipe := NewGeneralRecipe()
				generalRecipe.WithTree(tree)
				err = generalRecipe.Save(tmpdir)
				Expect(err).ToNot(HaveOccurred())

				generalRecipe.WithTree(nil)
				Expect(generalRecipe.Tree()).To(BeNil())

				err = generalRecipe.Load(tmpdir)
				Expect(err).ToNot(HaveOccurred())
				Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

				Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(10))

				for _, pid := range tree.GetPackageSet().GetPackages() {
					p, err := tree.GetPackageSet().GetPackage(pid)
					Expect(err).ToNot(HaveOccurred())
					Expect(p.GetName()).To(ContainSubstring("pinentry"))
				}
			})
		})

		Context("Simple solving with the fixture tree", func() {
			It("writes and reads back the same tree", func() {
				tmpdir, err := ioutil.TempDir("", "tree")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				gb := gentoo.NewGentooBuilder(&gentoo.SimpleEbuildParser{}, 20, dbType)
				tree, err := gb.Generate("../../tests/fixtures/overlay")
				Expect(err).ToNot(HaveOccurred())
				defer func() {
					Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))

				pack, err := tree.FindPackage(&pkg.DefaultPackage{Name: "pinentry", Version: "1.0.0-r2", Category: "app-crypt"}) // Note: the definition depends on pinentry-base without an explicit version
				Expect(err).ToNot(HaveOccurred())
				world, err := tree.World()
				Expect(err).ToNot(HaveOccurred())

				s := solver.NewSolver([]pkg.Package{}, world)
				solution, err := s.Install([]pkg.Package{pack})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(3))

				var allSol string
				for _, sol := range solution {
					allSol = allSol + "\n" + sol.ToString()
				}

				Expect(allSol).To(ContainSubstring("app-crypt/pinentry-base  installed: true"))
				Expect(allSol).To(ContainSubstring("app-crypt/pinentry 1.1.0-r2 installed: false"))
				Expect(allSol).To(ContainSubstring("app-crypt/pinentry 1.0.0-r2 installed: true"))
			})
		})
	}
})
