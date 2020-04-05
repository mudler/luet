// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package version_test

import (
	gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"

	. "github.com/mudler/luet/pkg/versioner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Versions", func() {

	Context("Versions Parser1", func() {
		v, err := ParseVersion(">=1.0")
		It("ParseVersion1", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser2", func() {
		v, err := ParseVersion(">1.0")
		It("ParseVersion2", func() {
			var c PkgSelectorCondition = PkgCondGreater
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser3", func() {
		v, err := ParseVersion("<=1.0")
		It("ParseVersion3", func() {
			var c PkgSelectorCondition = PkgCondLessEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser4", func() {
		v, err := ParseVersion("<1.0")
		It("ParseVersion4", func() {
			var c PkgSelectorCondition = PkgCondLess
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser5", func() {
		v, err := ParseVersion("=1.0")
		It("ParseVersion5", func() {
			var c PkgSelectorCondition = PkgCondEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser6", func() {
		v, err := ParseVersion("!1.0")
		It("ParseVersion6", func() {
			var c PkgSelectorCondition = PkgCondNot
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser7", func() {
		v, err := ParseVersion("")
		It("ParseVersion7", func() {
			var c PkgSelectorCondition = PkgCondInvalid
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal(""))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser8", func() {
		v, err := ParseVersion("=12.1.0.2_p1")
		It("ParseVersion8", func() {
			var c PkgSelectorCondition = PkgCondEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("12.1.0.2"))
			Expect(v.VersionSuffix).Should(Equal("_p1"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser9", func() {
		v, err := ParseVersion(">=0.0.20190406.4.9.172-r1")
		It("ParseVersion9", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.0.20190406.4.9.172"))
			Expect(v.VersionSuffix).Should(Equal("-r1"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser10", func() {
		v, err := ParseVersion(">=0.0.20190406.4.9.172_alpha")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.0.20190406.4.9.172"))
			Expect(v.VersionSuffix).Should(Equal("_alpha"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser11 - semver", func() {
		v, err := ParseVersion("0.1.0+0")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.1.0+0"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser12 - semver", func() {
		v, err := ParseVersion(">=0.1.0_alpha+AB")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.1.0+AB"))
			Expect(v.VersionSuffix).Should(Equal("_alpha"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser13 - semver", func() {
		v, err := ParseVersion(">=0.1.0_alpha+0.1.22")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.1.0+0.1.22"))
			Expect(v.VersionSuffix).Should(Equal("_alpha"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser14 - semver", func() {
		v, err := ParseVersion(">=0.1.0_alpha+0.1.22")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondGreaterEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.1.0+0.1.22"))
			Expect(v.VersionSuffix).Should(Equal("_alpha"))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser15 - semver", func() {
		v, err := ParseVersion("<=0.3.222.4.5+AB")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondLessEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("0.3.222.4.5+AB"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Versions Parser16 - semver", func() {
		v, err := ParseVersion("<=1.0.29+pre2_p20191024")
		It("ParseVersion10", func() {
			var c PkgSelectorCondition = PkgCondLessEqual
			Expect(err).Should(BeNil())
			Expect(v.Version).Should(Equal("1.0.29+pre2_p20191024"))
			Expect(v.VersionSuffix).Should(Equal(""))
			Expect(v.Condition).Should(Equal(c))
		})
	})

	Context("Selector1", func() {
		v1, err := ParseVersion(">=0.0.20190406.4.9.172-r1")
		v2, err2 := ParseVersion("1.0.111")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector1", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	Context("Selector2", func() {
		v1, err := ParseVersion(">=0.0.20190406.4.9.172-r1")
		v2, err2 := ParseVersion("0")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector2", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(false))
		})
	})

	Context("Selector3", func() {
		v1, err := ParseVersion(">0")
		v2, err2 := ParseVersion("0.0.40-alpha")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector3", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	Context("Selector4", func() {
		v1, err := ParseVersion(">0")
		v2, err2 := ParseVersion("")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector4", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	Context("Selector5", func() {
		v1, err := ParseVersion(">0.1.0+0.4")
		v2, err2 := ParseVersion("0.1.0+0.3")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector5", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(false))
		})
	})

	Context("Selector6", func() {
		v1, err := ParseVersion(">=0.1.0+0.4")
		v2, err2 := ParseVersion("0.1.0+0.5")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector6", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	PContext("Selector7", func() {
		v1, err := ParseVersion(">0.1.0+0.4")
		v2, err2 := ParseVersion("0.1.0+0.5")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector7", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	Context("Selector8", func() {
		v1, err := ParseVersion(">=0")
		v2, err2 := ParseVersion("1.0.29+pre2_p20191024")
		match, err3 := PackageAdmit(v1, v2)
		It("Selector8", func() {
			Expect(err).Should(BeNil())
			Expect(err2).Should(BeNil())
			Expect(err3).Should(BeNil())
			Expect(match).Should(Equal(true))
		})
	})

	Context("Condition Converter 1", func() {
		gp, err := gentoo.ParsePackageStr("=layer/build-1.0")
		var cond gentoo.PackageCond = gentoo.PkgCondEqual
		It("Converter1", func() {
			Expect(err).Should(BeNil())
			Expect((*gp).Condition).Should(Equal(cond))
			Expect(PkgSelectorConditionFromInt((*gp).Condition.Int()).String()).Should(Equal(""))
		})
	})

})
