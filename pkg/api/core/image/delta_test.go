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

var _ = Describe("Delta", func() {
	Context("Generates deltas of images", func() {
		It("computes delta", func() {
			ref, err := name.ParseReference("alpine")
			Expect(err).ToNot(HaveOccurred())

			img, err := daemon.Image(ref)
			Expect(err).ToNot(HaveOccurred())

			layers, err := Delta(img, img)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(layers.Changes)).To(Equal(0))
			Expect(len(layers.Additions)).To(Equal(0))
			Expect(len(layers.Deletions)).To(Equal(0))
		})

		Context("ExtractDeltaFiles", func() {
			ctx := types.NewContext()
			var tmpfile *os.File
			var ref, ref2 name.Reference
			var img, img2 v1.Image
			var diff ImageDiff
			var err error

			BeforeEach(func() {
				ctx = types.NewContext()

				tmpfile, err = ioutil.TempFile("", "delta")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpfile.Name()) // clean up

				ref, err = name.ParseReference("alpine")
				Expect(err).ToNot(HaveOccurred())

				ref2, err = name.ParseReference("golang:alpine")
				Expect(err).ToNot(HaveOccurred())

				img, err = daemon.Image(ref)
				Expect(err).ToNot(HaveOccurred())
				img2, err = daemon.Image(ref2)
				Expect(err).ToNot(HaveOccurred())

				diff, err = Delta(img, img2)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(diff.Additions) > 0).To(BeTrue())
				Expect(len(diff.Changes) > 0).To(BeTrue())
				Expect(len(diff.Deletions) == 0).To(BeTrue())
			})

			It("Extract all deltas", func() {
				_, tmpdir, err := Extract(
					ctx,
					img2,
					ExtractDeltaFiles(ctx, diff, []string{}, []string{}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				Expect(file.Exists(filepath.Join(tmpdir, "root", ".cache"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "bin", "sh"))).To(BeFalse())
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go", "bin"))).To(BeTrue())
			})

			It("Extract deltas and excludes /usr/local/go", func() {
				_, tmpdir, err := Extract(
					ctx,
					img2,
					ExtractDeltaFiles(ctx, diff, []string{}, []string{"usr/local/go"}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go"))).To(BeFalse())
			})
			It("Extract deltas and excludes /usr/local/go/bin, but includes /usr/local/go", func() {
				_, tmpdir, err := Extract(
					ctx,
					img2,
					ExtractDeltaFiles(ctx, diff, []string{"usr/local/go"}, []string{"usr/local/go/bin"}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go", "bin"))).To(BeFalse())
			})

			It("Extract deltas and includes /usr/local/go", func() {
				_, tmpdir, err := Extract(
					ctx,
					img2,
					ExtractDeltaFiles(ctx, diff, []string{"usr/local/go"}, []string{}),
				)
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tmpdir) // clean up

				Expect(file.Exists(filepath.Join(tmpdir, "usr", "local", "go"))).To(BeTrue())
				Expect(file.Exists(filepath.Join(tmpdir, "root", ".cache"))).To(BeFalse())
			})
		})
	})
})
