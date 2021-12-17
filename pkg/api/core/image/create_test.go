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
	"os"
	"path/filepath"
	"runtime"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/helpers/file"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	Context("Creates an OCI image from a standard tar", func() {
		It("creates an image which is loadable", func() {
			ctx := context.NewContext()

			dst, err := ctx.TempFile("dst")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dst.Name())
			srcTar, err := ctx.TempFile("srcTar")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(srcTar.Name())

			b := backend.NewSimpleDockerBackend(ctx)

			b.DownloadImage(backend.Options{ImageName: "alpine"})
			img, err := b.ImageReference("alpine", false)
			Expect(err).ToNot(HaveOccurred())

			_, dir, err := Extract(ctx, img, nil)
			Expect(err).ToNot(HaveOccurred())

			defer os.RemoveAll(dir)

			Expect(file.Touch(filepath.Join(dir, "test"))).ToNot(HaveOccurred())
			Expect(file.Exists(filepath.Join(dir, "bin"))).To(BeTrue())

			a := artifact.NewPackageArtifact(srcTar.Name())
			a.Compress(dir, 1)

			// Unfortunately there is no other easy way to test this
			err = CreateTar(srcTar.Name(), dst.Name(), "testimage", runtime.GOARCH, runtime.GOOS)
			Expect(err).ToNot(HaveOccurred())

			b.LoadImage(dst.Name())

			Expect(b.ImageExists("testimage")).To(BeTrue())

			img, err = b.ImageReference("testimage", false)
			Expect(err).ToNot(HaveOccurred())

			_, dir, err = Extract(ctx, img, nil)
			Expect(err).ToNot(HaveOccurred())

			defer os.RemoveAll(dir)
			Expect(file.Exists(filepath.Join(dir, "bin"))).To(BeTrue())
			Expect(file.Exists(filepath.Join(dir, "test"))).To(BeTrue())
		})
	})
})
