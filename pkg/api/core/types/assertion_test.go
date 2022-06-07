// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types_test

import (
	types "github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/database"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assertions", func() {
	Context("Ordering", func() {
		It("orders them correctly", func() {
			foo := &types.Package{Name: "foo", PackageRequires: []*types.Package{{Name: "bar"}}}
			assertions := types.PackagesAssertions{
				{Package: foo},
				{Package: &types.Package{Name: "baz", PackageRequires: []*types.Package{{Name: "bar"}}}},
				{Package: &types.Package{Name: "bar", PackageRequires: []*types.Package{{}}}},
			}

			ordered_old, err := assertions.Order(database.NewInMemoryDatabase(false), foo.GetFingerPrint())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(ordered_old[0].Package.Name).To(Equal("bar"))

			ordered, err := assertions.EnsureOrder(database.NewInMemoryDatabase(false))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(ordered)).To(Equal(3))

			Expect(ordered[0].Package.Name).To(Equal("bar"))
		})

		It("orders them correctly", func() {
			foo := &types.Package{Name: "foo", PackageRequires: []*types.Package{{Name: "bar"}}}
			assertions := types.PackagesAssertions{
				{Package: foo},
				{Package: &types.Package{Name: "baz2", PackageRequires: []*types.Package{{Name: "foobaz"}}}},
				{Package: &types.Package{Name: "baz", PackageRequires: []*types.Package{{Name: "bar"}}}},
				{Package: &types.Package{Name: "bar", PackageRequires: []*types.Package{{}}}},
				{Package: &types.Package{Name: "foobaz", PackageRequires: []*types.Package{{}}}},
			}

			ordered_old, err := assertions.Order(database.NewInMemoryDatabase(false), foo.GetFingerPrint())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(ordered_old[0].Package.Name).To(Equal("bar"))
			Expect(ordered_old[1].Package.Name).ToNot(Equal("foobaz"))

			ordered, err := assertions.EnsureOrder(database.NewInMemoryDatabase(false))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(ordered)).To(Equal(5))

			Expect(ordered[0].Package.Name).To(Equal("bar"))
			Expect(ordered[1].Package.Name).To(Equal("foobaz"))
		})

		It("orders them correctly", func() {
			foo := &types.Package{Name: "foo", PackageRequires: []*types.Package{{Name: "bar"}}}
			assertions := types.PackagesAssertions{
				{Package: foo},
				{Package: &types.Package{Name: "bazbaz2", PackageRequires: []*types.Package{{Name: "baz2"}}}},
				{Package: &types.Package{Name: "baz2", PackageRequires: []*types.Package{{Name: "foobaz"}, {Name: "baz"}}}},
				{Package: &types.Package{Name: "baz", PackageRequires: []*types.Package{{Name: "bar"}}}},
				{Package: &types.Package{Name: "bar", PackageRequires: []*types.Package{{}}}},
				{Package: &types.Package{Name: "foobaz", PackageRequires: []*types.Package{{}}}},
			}

			ordered_old, err := assertions.Order(database.NewInMemoryDatabase(false), foo.GetFingerPrint())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(ordered_old[0].Package.Name).To(Equal("bar"))
			Expect(ordered_old[1].Package.Name).ToNot(Equal("foobaz"))

			ordered, err := assertions.EnsureOrder(database.NewInMemoryDatabase(false))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(ordered)).To(Equal(6))

			Expect(ordered[0].Package.Name).To(Or(Equal("foobaz"), Equal("bar")))
			Expect(ordered[1].Package.Name).To(Or(Equal("foobaz"), Equal("bar")))
			Expect(ordered[2].Package.Name).To(Or(Equal("foo"), Equal("baz")))
			Expect(ordered[3].Package.Name).To(Or(Equal("foo"), Equal("baz")))
			Expect(ordered[4].Package.Name).To(Equal("baz2"))
			Expect(ordered[5].Package.Name).To(Equal("bazbaz2"))

		})
	})
})
