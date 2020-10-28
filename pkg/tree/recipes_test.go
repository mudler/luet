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
					Expect(tree.Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackages())).To(Equal(10))

				generalRecipe := NewGeneralRecipe(tree)
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
					Expect(tree.Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackages())).To(Equal(10))

				generalRecipe := NewGeneralRecipe(tree)
				err = generalRecipe.Save(tmpdir)
				Expect(err).ToNot(HaveOccurred())

				db := pkg.NewInMemoryDatabase(false)
				generalRecipe = NewGeneralRecipe(db)

				generalRecipe.WithDatabase(nil)
				Expect(generalRecipe.GetDatabase()).To(BeNil())

				err = generalRecipe.Load(tmpdir)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(generalRecipe.GetDatabase().GetPackages())).To(Equal(10))

				for _, p := range tree.World() {
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
					Expect(tree.Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackages())).To(Equal(10))

				pack, err := tree.FindPackage(&pkg.DefaultPackage{
					Name:     "pinentry",
					Version:  "1.0.0-r2",
					Category: "app-crypt",
				}) // Note: the definition depends on pinentry-base without an explicit version
				Expect(err).ToNot(HaveOccurred())

				s := solver.NewSolver(solver.Options{Type: solver.SingleCoreSimple}, pkg.NewInMemoryDatabase(false), tree, tree)
				solution, err := s.Install([]pkg.Package{pack})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(solution)).To(Equal(33))

				var allSol string
				for _, sol := range solution {
					allSol = allSol + "\n" + sol.ToString()
				}

				Expect(allSol).To(ContainSubstring("app-crypt/pinentry-base 1.0.0 installed"))
				Expect(allSol).To(ContainSubstring("app-crypt/pinentry 1.1.0-r2 not installed"))
				Expect(allSol).To(ContainSubstring("app-crypt/pinentry 1.0.0-r2 installed"))
			})
		})
	}
})
