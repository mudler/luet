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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	daemon "github.com/google/go-containerregistry/pkg/v1/daemon"
	. "github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/helpers/file"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extract", func() {

	Context("extract files from images", func() {
		Context("ExtractFiles", func() {
			ctx := types.NewContext()
			var tmpfile *os.File
			var ref name.Reference
			var img v1.Image
			var err error

			BeforeEach(func() {
				ctx = types.NewContext()

				tmpfile, err = ioutil.TempFile("", "extract")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpfile.Name()) // clean up

				ref, err = name.ParseReference("alpine")
				Expect(err).ToNot(HaveOccurred())

				img, err = daemon.Image(ref)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Extract all files", func() {
				_, tmpdir, err := Extract(
					ctx,
					img,
					true,
					ExtractFiles(ctx, "", []string{}, []string{}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				Expect(file.Exists(filepath.Join(tmpdir, "usr", "bin"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "bin", "sh"))).To(BeTrue())
			})

			It("Extract specific dir", func() {
				_, tmpdir, err := Extract(
					ctx,
					img,
					true,
					ExtractFiles(ctx, "/usr", []string{}, []string{}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "sbin"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "bin"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "bin", "sh"))).To(BeFalse())
			})

			It("Extract a dir with includes/excludes", func() {
				_, tmpdir, err := Extract(
					ctx,
					img,
					true,
					ExtractFiles(ctx, "/usr", []string{"bin"}, []string{"sbin"}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				Expect(file.Exists(filepath.Join(tmpdir, "usr", "bin"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "bin", "sh"))).To(BeFalse())
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "sbin"))).To(BeFalse())
			})

			It("Extract with includes/excludes", func() {
				_, tmpdir, err := Extract(
					ctx,
					img,
					true,
					ExtractFiles(ctx, "", []string{"/usr|/usr/bin"}, []string{"^/bin"}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				Expect(file.Exists(filepath.Join(tmpdir, "usr", "bin"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "bin", "sh"))).To(BeFalse())
			})
		})
	})
})
