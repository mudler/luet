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

package artifact_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/api/core/types/artifact"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache", func() {
	Context("CacheID", func() {
		It("Get and retrieve files", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up

			tmpdirartifact, err := ioutil.TempDir(os.TempDir(), "testartifact")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdirartifact) // clean up

			err = ioutil.WriteFile(filepath.Join(tmpdirartifact, "foo"), []byte(string("foo")), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			a := NewPackageArtifact(filepath.Join(tmpdir, "foo.tar.gz"))
			err = a.Compress(tmpdirartifact, 1)
			Expect(err).ToNot(HaveOccurred())

			cache := NewCache(tmpdir)

			// Put an artifact in the cache and retrieve it later
			// the artifact is NOT hashed so it is referenced just by the path in the cache
			_, _, err = cache.Put(a)
			Expect(err).ToNot(HaveOccurred())

			path, err := cache.Get(a)
			Expect(err).ToNot(HaveOccurred())

			b := NewPackageArtifact(path)
			ctx := context.NewContext()
			err = b.Unpack(ctx, tmpdir, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(fileHelper.Exists(filepath.Join(tmpdir, "foo"))).To(BeTrue())

			bb, err := ioutil.ReadFile(filepath.Join(tmpdir, "foo"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(bb)).To(Equal("foo"))

			// After the artifact is hashed, the fingerprint mutates so the cache doesn't see it hitting again
			// the test we did above fails as we expect to.
			a.Hash()
			_, err = cache.Get(a)
			Expect(err).To(HaveOccurred())

			a.CompileSpec = &types.LuetCompilationSpec{Package: &types.Package{Name: "foo", Category: "bar"}}
			_, _, err = cache.Put(a)
			Expect(err).ToNot(HaveOccurred())

			c := NewPackageArtifact(filepath.Join(tmpdir, "foo.tar.gz"))
			c.Hash()
			c.CompileSpec = &types.LuetCompilationSpec{Package: &types.Package{Name: "foo", Category: "bar"}}
			_, err = cache.Get(c)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
