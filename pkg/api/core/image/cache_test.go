// Copyright © 2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package image_test

import (
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/helpers/file"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache", func() {

	ctx := context.NewContext()
	Context("used as k/v store", func() {

		cache := &Cache{}
		var dir string

		BeforeEach(func() {
			ctx = context.NewContext()
			var err error
			dir, err = ctx.TempDir("foo")
			Expect(err).ToNot(HaveOccurred())
			cache = NewCache(dir, 10*1024*1024, 1) // 10MB Cache when upgrading to files. Max volatile memory of 1 row.
		})

		AfterEach(func() {
			cache.Clean()
		})

		It("does handle automatically memory upgrade", func() {
			cache.Set("foo", "bar")
			v, found := cache.Get("foo")
			Expect(found).To(BeTrue())
			Expect(v).To(Equal("bar"))
			Expect(file.Exists(filepath.Join(dir, "foo"))).To(BeFalse())
			cache.Set("baz", "bar")
			Expect(file.Exists(filepath.Join(dir, "foo"))).To(BeTrue())
			Expect(file.Exists(filepath.Join(dir, "baz"))).To(BeTrue())
			v, found = cache.Get("foo")
			Expect(found).To(BeTrue())
			Expect(v).To(Equal("bar"))

			Expect(cache.Count()).To(Equal(2))
		})

		It("does CRUD", func() {
			cache.Set("foo", "bar")

			v, found := cache.Get("foo")
			Expect(found).To(BeTrue())
			Expect(v).To(Equal("bar"))

			hit := false
			cache.All(func(c CacheResult) {
				hit = true
				Expect(c.Key()).To(Equal("foo"))
				Expect(c.Value()).To(Equal("bar"))
			})
			Expect(hit).To(BeTrue())

		})

		It("Unmarshals values", func() {
			type testStruct struct {
				Test string
			}

			cache.SetValue("foo", &testStruct{Test: "baz"})

			n := &testStruct{}

			cache.All(func(cr CacheResult) {
				err := cr.Unmarshal(n)
				Expect(err).ToNot(HaveOccurred())

			})
			Expect(n.Test).To(Equal("baz"))
		})
	})
})
