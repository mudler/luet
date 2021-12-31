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
	"io/ioutil"
	"os"
	"strconv"

	. "github.com/mudler/luet/pkg/package"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Database  benchmark", func() {

	Context("BoltDB", func() {

		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

		tmpfile, _ := ioutil.TempFile(os.TempDir(), "tests")
		defer os.Remove(tmpfile.Name()) // clean up
		var db PackageSet

		BeforeEach(func() {

			tmpfile, _ = ioutil.TempFile(os.TempDir(), "tests")
			defer os.Remove(tmpfile.Name()) // clean up
			db = NewBoltDatabase(tmpfile.Name())
			if os.Getenv("BENCHMARK_TESTS") != "true" {
				Skip("BENCHMARK_TESTS not enabled")
			}
		})

		Measure("it should be fast in computing world from a 50000 dataset", func(b Benchmarker) {
			for i := 0; i < 50000; i++ {
				a = NewPackage("A"+strconv.Itoa(i), ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

				_, err := db.CreatePackage(a)
				Expect(err).ToNot(HaveOccurred())
			}
			runtime := b.Time("runtime", func() {
				packs := db.World()
				Expect(len(packs)).To(Equal(50000))
			})

			Ω(runtime.Seconds()).Should(BeNumerically("<", 30), "World() shouldn't take too long.")

		}, 1)

		Measure("it should be fast in computing world from a 100000 dataset", func(b Benchmarker) {
			for i := 0; i < 100000; i++ {
				a = NewPackage("A"+strconv.Itoa(i), ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

				_, err := db.CreatePackage(a)
				Expect(err).ToNot(HaveOccurred())
			}
			runtime := b.Time("runtime", func() {
				packs := db.World()
				Expect(len(packs)).To(Equal(100000))
			})

			Ω(runtime.Seconds()).Should(BeNumerically("<", 30), "World() shouldn't take too long.")

		}, 1)
	})

	Context("InMemory", func() {

		a := NewPackage("A", ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

		tmpfile, _ := ioutil.TempFile(os.TempDir(), "tests")
		defer os.Remove(tmpfile.Name()) // clean up
		var db PackageSet

		BeforeEach(func() {

			tmpfile, _ = ioutil.TempFile(os.TempDir(), "tests")
			defer os.Remove(tmpfile.Name()) // clean up
			db = NewInMemoryDatabase(false)
			if os.Getenv("BENCHMARK_TESTS") != "true" {
				Skip("BENCHMARK_TESTS not enabled")
			}
		})

		Measure("it should be fast in computing world from a 100000 dataset", func(b Benchmarker) {

			runtime := b.Time("runtime", func() {
				for i := 0; i < 100000; i++ {
					a = NewPackage("A"+strconv.Itoa(i), ">=1.0", []*DefaultPackage{}, []*DefaultPackage{})

					_, err := db.CreatePackage(a)
					Expect(err).ToNot(HaveOccurred())
				}
				packs := db.World()
				Expect(len(packs)).To(Equal(100000))
			})

			Ω(runtime.Seconds()).Should(BeNumerically("<", 10), "World() shouldn't take too long.")

		}, 2)
	})
})
