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

package compiler_test

import (
	. "github.com/mudler/luet/pkg/compiler"
	. "github.com/mudler/luet/pkg/compiler/backend"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker image diffs", func() {
	var b CompilerBackend

	BeforeEach(func() {
		b = NewSimpleDockerBackend()
	})

	Context("Generate diffs from docker images", func() {
		It("Detect no changes", func() {
			opts := Options{
				ImageName: "alpine:latest",
			}
			err := b.DownloadImage(opts)
			Expect(err).ToNot(HaveOccurred())

			layers, err := GenerateChanges(b, opts, opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(layers)).To(Equal(1))
			Expect(len(layers[0].Diffs.Additions)).To(Equal(0))
			Expect(len(layers[0].Diffs.Changes)).To(Equal(0))
			Expect(len(layers[0].Diffs.Deletions)).To(Equal(0))
		})

		It("Detects additions and changed files", func() {
			err := b.DownloadImage(Options{
				ImageName: "quay.io/mocaccino/micro",
			})
			Expect(err).ToNot(HaveOccurred())
			err = b.DownloadImage(Options{
				ImageName: "quay.io/mocaccino/extra",
			})
			Expect(err).ToNot(HaveOccurred())

			layers, err := GenerateChanges(b, Options{
				ImageName: "quay.io/mocaccino/micro",
			}, Options{
				ImageName: "quay.io/mocaccino/extra",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(layers)).To(Equal(1))

			Expect(len(layers[0].Diffs.Changes) > 0).To(BeTrue())
			Expect(len(layers[0].Diffs.Changes[0].Name) > 0).To(BeTrue())
			Expect(layers[0].Diffs.Changes[0].Size > 0).To(BeTrue())

			Expect(len(layers[0].Diffs.Additions) > 0).To(BeTrue())
			Expect(len(layers[0].Diffs.Additions[0].Name) > 0).To(BeTrue())
			Expect(layers[0].Diffs.Additions[0].Size > 0).To(BeTrue())

			Expect(len(layers[0].Diffs.Deletions)).To(Equal(0))
		})
	})
})
