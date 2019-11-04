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
	. "github.com/mudler/luet/pkg/compiler"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Spec", func() {
	Context("Simple package build definition", func() {
		It("Loads it correctly", func() {
			generalRecipe := tree.NewGeneralRecipe()

			err := generalRecipe.Load("../../tests/fixtures/buildtree")
			Expect(err).ToNot(HaveOccurred())
			Expect(generalRecipe.Tree()).ToNot(BeNil()) // It should be populated back at this point

			Expect(len(generalRecipe.Tree().GetPackageSet().GetPackages())).To(Equal(1))

			compiler := NewLuetCompiler(nil, generalRecipe.Tree())
			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "enman", Category: "app-admin", Version: "1.4.0"})
			Expect(err).ToNot(HaveOccurred())

			lspec, ok := spec.(*LuetCompilationSpec)
			Expect(ok).To(BeTrue())

			Expect(lspec.Steps).To(Equal([]string{"echo foo", "bar"}))
			Expect(lspec.Image).To(Equal("luet/base"))
		})

	})
})
