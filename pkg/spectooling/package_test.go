// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>,
//                       Daniele Rondina <geaaru@sabayonlinux.org>
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

package spectooling_test

import (
	pkg "github.com/mudler/luet/pkg/package"
	. "github.com/mudler/luet/pkg/spectooling"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Spec Tooling", func() {
	Context("Conversion1", func() {

		b := pkg.NewPackage("B", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		c := pkg.NewPackage("C", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		d := pkg.NewPackage("D", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		p1 := pkg.NewPackage("A", "1.0", []*pkg.DefaultPackage{b, c}, []*pkg.DefaultPackage{d})
		virtual := pkg.NewPackage("E", "1.0", []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
		virtual.SetCategory("virtual")
		p1.Provides = []*pkg.DefaultPackage{virtual}
		p1.AddLabel("label1", "value1")
		p1.AddLabel("label2", "value2")
		p1.SetDescription("Package1")
		p1.SetCategory("cat1")
		p1.SetLicense("GPL")
		p1.AddURI("https://github.com/mudler/luet")
		p1.AddUse("systemd")
		It("Convert pkg1", func() {
			res := NewDefaultPackageSanitized(p1)
			expected_res := &DefaultPackageSanitized{
				Name:     "A",
				Version:  "1.0",
				Category: "cat1",
				PackageRequires: []*DefaultPackageSanitized{
					&DefaultPackageSanitized{
						Name:    "B",
						Version: "1.0",
					},
					&DefaultPackageSanitized{
						Name:    "C",
						Version: "1.0",
					},
				},
				PackageConflicts: []*DefaultPackageSanitized{
					&DefaultPackageSanitized{
						Name:    "D",
						Version: "1.0",
					},
				},
				Provides: []*DefaultPackageSanitized{
					&DefaultPackageSanitized{
						Name:     "E",
						Category: "virtual",
						Version:  "1.0",
					},
				},
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Description: "Package1",
				License:     "GPL",
				Uri:         []string{"https://github.com/mudler/luet"},
				UseFlags:    []string{"systemd"},
			}

			Expect(res).To(Equal(expected_res))
		})

	})
})
