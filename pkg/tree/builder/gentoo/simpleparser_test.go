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

package gentoo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	. "github.com/mudler/luet/pkg/tree/builder/gentoo"
)

var _ = Describe("GentooBuilder", func() {

	Context("Parse RDEPEND1", func() {

		rdepend := `
	app-crypt/sbsigntools
	x11-themes/sabayon-artwork-grub
	sys-boot/os-prober
	app-arch/xz-utils
	>=sys-libs/ncurses-5.2-r5:0=
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(5))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sabayon-artwork-grub",
						Category: "x11-themes",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep5", func() {
			Expect(*gr.Dependencies[4]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

	})

	Context("Parse RDEPEND2", func() {

		rdepend := `
	app-crypt/sbsigntools
	x11-themes/sabayon-artwork-grub
	sys-boot/os-prober
	app-arch/xz-utils
	>=sys-libs/ncurses-5.2-r5:0=
	mount? ( sys-fs/fuse )
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(6))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sabayon-artwork-grub",
						Category: "x11-themes",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep5", func() {
			Expect(*gr.Dependencies[4]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

		It("Check dep6", func() {
			Expect(*gr.Dependencies[5]).Should(Equal(
				GentooDependency{
					Use:          "mount",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps: []*_gentoo.GentooPackage{&_gentoo.GentooPackage{
						Name:     "fuse",
						Category: "sys-fs",
						Slot:     "0",
					}},
					Dep: nil,
				},
			))
		})

	})

	Context("Parse RDEPEND3", func() {

		rdepend := `
	app-crypt/sbsigntools
	x11-themes/sabayon-artwork-grub
	sys-boot/os-prober
	app-arch/xz-utils
	>=sys-libs/ncurses-5.2-r5:0=
	mount? ( sys-fs/fuse =sys-apps/pmount-0.9.99_alpha-r5:= )
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(6))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sabayon-artwork-grub",
						Category: "x11-themes",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep5", func() {
			Expect(*gr.Dependencies[4]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

		It("Check dep6", func() {
			Expect(*gr.Dependencies[5]).Should(Equal(
				GentooDependency{
					Use:          "mount",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps: []*_gentoo.GentooPackage{
						&_gentoo.GentooPackage{
							Name:     "fuse",
							Category: "sys-fs",
							Slot:     "0",
						},
						&_gentoo.GentooPackage{
							Name:          "pmount",
							Category:      "sys-apps",
							Condition:     _gentoo.PkgCondEqual,
							Version:       "0.9.99",
							VersionSuffix: "_alpha-r5",
							Slot:          "=",
						},
					},
					Dep: nil,
				},
			))
		})

	})

	Context("Parse RDEPEND4", func() {

		rdepend := `
	app-crypt/sbsigntools
	x11-themes/sabayon-artwork-grub
	sys-boot/os-prober
	app-arch/xz-utils
	>=sys-libs/ncurses-5.2-r5:0=
	!mount? ( sys-fs/fuse =sys-apps/pmount-0.9.99_alpha-r5:= )
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(6))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sabayon-artwork-grub",
						Category: "x11-themes",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep5", func() {
			Expect(*gr.Dependencies[4]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

		It("Check dep6", func() {
			Expect(*gr.Dependencies[5]).Should(Equal(
				GentooDependency{
					Use:          "mount",
					UseCondition: _gentoo.PkgCondNot,
					SubDeps: []*_gentoo.GentooPackage{
						&_gentoo.GentooPackage{
							Name:     "fuse",
							Category: "sys-fs",
							Slot:     "0",
						},
						&_gentoo.GentooPackage{
							Name:          "pmount",
							Category:      "sys-apps",
							Condition:     _gentoo.PkgCondEqual,
							Version:       "0.9.99",
							VersionSuffix: "_alpha-r5",
							Slot:          "=",
						},
					},
					Dep: nil,
				},
			))
		})

	})

	Context("Parse RDEPEND5", func() {

		rdepend := `
	app-crypt/sbsigntools
	>=sys-libs/ncurses-5.2-r5:0=
	mount? (
		sys-fs/fuse
		=sys-apps/pmount-0.9.99_alpha-r5:=
	)
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(3))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

		It("Check dep3", func() {
			Expect(*gr.Dependencies[2]).Should(Equal(
				GentooDependency{
					Use:          "mount",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps: []*_gentoo.GentooPackage{
						&_gentoo.GentooPackage{
							Name:     "fuse",
							Category: "sys-fs",
							Slot:     "0",
						},
						&_gentoo.GentooPackage{
							Name:          "pmount",
							Category:      "sys-apps",
							Condition:     _gentoo.PkgCondEqual,
							Version:       "0.9.99",
							VersionSuffix: "_alpha-r5",
							Slot:          "=",
						},
					},
					Dep: nil,
				},
			))
		})

	})

	Context("Parse RDEPEND6", func() {

		rdepend := `
	app-crypt/sbsigntools
	>=sys-libs/ncurses-5.2-r5:0=
	mount? (
		sys-fs/fuse
		=sys-apps/pmount-0.9.99_alpha-r5:= )
`
		gr, err := ParseRDEPEND(rdepend)
		It("Check error", func() {
			Expect(err).Should(BeNil())
		})
		It("Check gr", func() {
			Expect(gr).ShouldNot(BeNil())
		})

		It("Check deps #", func() {
			Expect(len(gr.Dependencies)).Should(Equal(3))
		})

		It("Check dep1", func() {
			Expect(*gr.Dependencies[0]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:     "sbsigntools",
						Category: "app-crypt",
						Slot:     "0",
					},
				},
			))
		})

		It("Check dep2", func() {
			Expect(*gr.Dependencies[1]).Should(Equal(
				GentooDependency{
					Use:          "",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps:      make([]*_gentoo.GentooPackage, 0),
					Dep: &_gentoo.GentooPackage{
						Name:          "ncurses",
						Category:      "sys-libs",
						Slot:          "0=",
						Version:       "5.2",
						VersionSuffix: "-r5",
						Condition:     _gentoo.PkgCondGreaterEqual,
					},
				},
			))
		})

		It("Check dep3", func() {
			Expect(*gr.Dependencies[2]).Should(Equal(
				GentooDependency{
					Use:          "mount",
					UseCondition: _gentoo.PkgCondInvalid,
					SubDeps: []*_gentoo.GentooPackage{
						&_gentoo.GentooPackage{
							Name:     "fuse",
							Category: "sys-fs",
							Slot:     "0",
						},
						&_gentoo.GentooPackage{
							Name:          "pmount",
							Category:      "sys-apps",
							Condition:     _gentoo.PkgCondEqual,
							Version:       "0.9.99",
							VersionSuffix: "_alpha-r5",
							Slot:          "=",
						},
					},
					Dep: nil,
				},
			))
		})

	})

	Context("Simple test", func() {
		for _, dbType := range []MemoryDB{InMemory, BoltDB} {
			It("parses correctly deps", func() {
				gb := NewGentooBuilder(&SimpleEbuildParser{}, 20, dbType)
				tree, err := gb.Generate("../../../../tests/fixtures/overlay")
				Expect(err).ToNot(HaveOccurred())
				defer func() {
					Expect(tree.GetPackageSet().Clean()).ToNot(HaveOccurred())
				}()

				Expect(len(tree.GetPackageSet().GetPackages())).To(Equal(10))

				for _, pid := range tree.GetPackageSet().GetPackages() {
					p, err := tree.GetPackageSet().GetPackage(pid)
					Expect(err).ToNot(HaveOccurred())
					Expect(p.GetName()).To(ContainSubstring("pinentry"))
					Expect(p.GetVersion()).To(ContainSubstring("1."))
				}

			})
		}
	})

})
