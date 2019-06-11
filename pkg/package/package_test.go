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

package pkg_test

import (
	. "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Package", func() {
	Context("Simple package", func() {
		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a1 := NewPackage("A", "1.0", []*DefaultPackage{}, []*DefaultPackage{})
		a11 := NewPackage("A", "1.1", []*DefaultPackage{}, []*DefaultPackage{})
		a01 := NewPackage("A", "0.1", []*DefaultPackage{}, []*DefaultPackage{})
		It("Expands correctly", func() {
			lst, err := a.Expand([]Package{a1, a11, a01})
			Expect(err).ToNot(HaveOccurred())
			Expect(lst).To(ContainElement(a11))
			Expect(lst).To(ContainElement(a1))
			Expect(lst).ToNot(ContainElement(a01))
			Expect(len(lst)).To(Equal(2))
		})
	})
})
