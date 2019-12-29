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
	"io/ioutil"
	"os"

	. "github.com/mudler/luet/pkg/compiler"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checksum", func() {
	Context("Generation", func() {
		It("Compares successfully", func() {

			tmpdir, err := ioutil.TempDir("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up
			buildsum := Checksums{}
			definitionsum := Checksums{}
			definitionsum2 := Checksums{}

			Expect(len(buildsum)).To(Equal(0))
			Expect(len(definitionsum)).To(Equal(0))
			Expect(len(definitionsum2)).To(Equal(0))

			err = buildsum.Generate(NewPackageArtifact("../../tests/fixtures/layers/alpine/build.yaml"))
			Expect(err).ToNot(HaveOccurred())

			err = definitionsum.Generate(NewPackageArtifact("../../tests/fixtures/layers/alpine/definition.yaml"))
			Expect(err).ToNot(HaveOccurred())

			err = definitionsum2.Generate(NewPackageArtifact("../../tests/fixtures/layers/alpine/definition.yaml"))
			Expect(err).ToNot(HaveOccurred())

			Expect(len(buildsum)).To(Equal(1))
			Expect(len(definitionsum)).To(Equal(1))
			Expect(len(definitionsum2)).To(Equal(1))

			Expect(definitionsum.Compare(buildsum)).To(HaveOccurred())
			Expect(definitionsum.Compare(definitionsum2)).ToNot(HaveOccurred())
		})
	})

})
