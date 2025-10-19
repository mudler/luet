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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/tree"
)

var _ = Describe("Templated tree", func() {
	Context("Resolves correctly dependencies", func() {
		It("interpolates correctly templated requires", func() {
			db := pkg.NewInMemoryDatabase(false)
			generalRecipe := NewCompilerRecipe(db)
			tmpdir, err := os.MkdirTemp("", "package")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			err = generalRecipe.Load("../../tests/fixtures/template_requires")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(generalRecipe.GetDatabase().World())).To(Equal(7))

			foo, err := generalRecipe.GetDatabase().FindPackage(&types.Package{Name: "foo"})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(foo.GetRequires())).To(Equal(1))
			Expect(foo.GetRequires()[0].Name).To(Equal("bar"))

			baz, err := generalRecipe.GetDatabase().FindPackage(&types.Package{Name: "baz"})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(baz.GetRequires())).To(Equal(1))
			Expect(baz.GetRequires()[0].Name).To(Equal("foobar"))

			bazbaz, err := generalRecipe.GetDatabase().FindPackage(&types.Package{Name: "bazbaz"})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(bazbaz.GetRequires())).To(Equal(1))
			Expect(bazbaz.GetRequires()[0].Name).To(Equal("foobar"))

			foo, err = generalRecipe.GetDatabase().FindPackage(&types.Package{Name: "foo", Category: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(foo.GetRequires())).To(Equal(1))
			Expect(foo.GetRequires()[0].Name).To(Equal("bar"))

			baz, err = generalRecipe.GetDatabase().FindPackage(&types.Package{Name: "baz", Category: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(baz.GetRequires())).To(Equal(1))
			Expect(baz.GetRequires()[0].Name).To(Equal("foobar"))
		})
	})
})
