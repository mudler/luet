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
	"fmt"

	. "github.com/mudler/luet/pkg/versioner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Versioner", func() {
	Context("Invalid version", func() {
		versioner := DefaultVersioner()
		It("Sanitize", func() {
			sanitized := versioner.Sanitize("foo_bar")
			Expect(sanitized).Should(Equal("foo-bar"))
		})
	})

	Context("valid version", func() {
		versioner := DefaultVersioner()
		It("Validate", func() {
			err := versioner.Validate("1.0")
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("invalid version", func() {
		versioner := DefaultVersioner()
		It("Validate", func() {
			err := versioner.Validate("1.0_##")
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("Sorting", func() {
		versioner := DefaultVersioner()
		It("finds the correct ordering", func() {
			sorted := versioner.Sort([]string{"1.0", "0.1"})
			Expect(sorted).Should(Equal([]string{"0.1", "1.0"}))
		})
	})

	Context("Sorting with invalid characters", func() {
		versioner := DefaultVersioner()
		It("finds the correct ordering", func() {
			sorted := versioner.Sort([]string{"1.0_1", "0.1"})
			Expect(sorted).Should(Equal([]string{"0.1", "1.0_1"}))
		})
	})

	Context("Complex Sorting", func() {
		versioner := DefaultVersioner()
		It("finds the correct ordering", func() {
			sorted := versioner.Sort([]string{"1.0", "0.1", "0.22", "1.1", "1.9", "1.10", "11.1"})
			Expect(sorted).Should(Equal([]string{"0.1", "0.22", "1.0", "1.1", "1.9", "1.10", "11.1"}))
		})
	})

	Context("Sorting with +", func() {
		versioner := DefaultVersioner()
		It("finds the correct ordering", func() {
			sorted := versioner.Sort([]string{"1.0+1", "1.0+0", "0.1", "1.0+3", "1.0+2", "1.9"})
			Expect(sorted).Should(Equal([]string{"0.1", "1.0+0", "1.0+1", "1.0+2", "1.0+3", "1.9"}))
		})
	})

	// from: https://github.com/knqyf263/go-deb-version/blob/master/version_test.go#L8
	Context("Debian Sorting", func() {
		versioner := DefaultVersioner()
		It("finds the correct ordering", func() {
			sorted := versioner.Sort([]string{"2:7.4.052-1ubuntu3.1", "2:7.4.052-1ubuntu1", "2:7.4.052-1ubuntu2", "2:7.4.052-1ubuntu3"})
			Expect(sorted).Should(Equal([]string{"2:7.4.052-1ubuntu1", "2:7.4.052-1ubuntu2", "2:7.4.052-1ubuntu3", "2:7.4.052-1ubuntu3.1"}))
		})
	})

	It("finds the correct ordering", func() {
		versioner := DefaultVersioner()

		sorted := versioner.Sort([]string{"0.0.1-beta-9", "0.0.1-alpha08-9", "0.0.1-alpha07-9", "0.0.1-alpha07-8"})
		Expect(sorted).Should(Equal([]string{"0.0.1-alpha07-8", "0.0.1-alpha07-9", "0.0.1-alpha08-9", "0.0.1-beta-9"}))
	})
	It("finds the correct ordering", func() {
		versioner := DefaultVersioner()

		sorted := versioner.Sort([]string{"0.0.1-beta01", "0.0.1-alpha08", "0.0.1-alpha07"})
		Expect(sorted).Should(Equal([]string{"0.0.1-alpha07", "0.0.1-alpha08", "0.0.1-beta01"}))
	})

	Context("Matching a selector", func() {

		testCases := [][]string{
			{">=1", "2"},
			{"<=3", "2"},
			{">0", ""},
			{">0", "0.0.40-alpha"},
			{">=0.1.0+0.4", "0.1.0+0.5"},
			{">=0.0.20190406.4.9.172-r1", "1.0.111"},
			{">=0", "1.0.29+pre2_p20191024"},
			{">=0.1.0+4", "0.1.0+5"},
			{">0.1.0-4", "0.1.0-5"},
			{"<1.2.3-beta", "1.2.3-beta.1-1"},
			{"<1.2.3", "1.2.3-beta.1"},
			{">0.0.1-alpha07", "0.0.1-alpha07-8"},
			{">0.0.1-alpha07-1", "0.0.1-alpha07-8"},
			{">0.0.1-alpha07", "0.0.1-alpha08"},
		}
		versioner := DefaultVersioner()

		for i, t := range testCases {
			selector := testCases[i][0]
			version := testCases[i][1]
			It(fmt.Sprint(t), func() {

				Expect(versioner.ValidateSelector(version, selector)).Should(BeTrue())
			})
		}
	})

	Context("Not matching a selector", func() {

		testfalseCases := [][]string{
			{">0.0.1-alpha07", "0.0.1-alpha06"},
			{"<0.0.1-alpha07", "0.0.1-alpha08"},
			{">0.1.0+0.4", "0.1.0+0.3"},
			{">=0.0.20190406.4.9.172-r1", "0"},
			{"<=1", "2"},
			{">=3", "2"},
			{"<0", "0.0.40-alpha"},
			{"<0.1.0+0.4", "0.1.0+0.5"},
			{"<=0.0.20190406.4.9.172-r1", "1.0.111"},
			{"<0.1.0+4", "0.1.0+5"},
			{"<=0.1.0-4", "0.1.0-5"},
		}

		versioner := DefaultVersioner()

		for i, t := range testfalseCases {
			selector := testfalseCases[i][0]
			version := testfalseCases[i][1]
			It(fmt.Sprint(t), func() {
				Expect(versioner.ValidateSelector(version, selector)).Should(BeFalse())
			})
		}
	})

})
