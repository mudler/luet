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

package file_test

import (
	"os"
	"path/filepath"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helpers", func() {
	Context("Exists", func() {
		It("Detect existing and not-existing files", func() {
			Expect(fileHelper.Exists("../../tests/fixtures/buildtree/app-admin/enman/1.4.0/build.yaml")).To(BeTrue())
			Expect(fileHelper.Exists("../../tests/fixtures/buildtree/app-admin/enman/1.4.0/build.yaml.not.exists")).To(BeFalse())
		})
	})

	Context("DirectoryIsEmpty", func() {
		It("Detects empty directory", func() {
			testDir, err := os.MkdirTemp(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)
			Expect(fileHelper.DirectoryIsEmpty(testDir)).To(BeTrue())
		})
		It("Detects directory with files", func() {
			testDir, err := os.MkdirTemp(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)
			err = fileHelper.Touch(filepath.Join(testDir, "foo"))
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.DirectoryIsEmpty(testDir)).To(BeFalse())
		})
	})

	Context("Orders dir and files correctly", func() {
		It("puts files first and folders at end", func() {
			testDir, err := os.MkdirTemp(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			err = os.WriteFile(filepath.Join(testDir, "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(filepath.Join(testDir, "baz"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "bar"), 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(testDir, "bar", "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "baz2"), 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(testDir, "baz2", "foo"), []byte("test\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			ordered, notExisting := fileHelper.OrderFiles(testDir, []string{"bar", "baz", "bar/foo", "baz2", "foo", "baz2/foo", "notexisting"})

			Expect(ordered).To(Equal([]string{"baz", "bar/foo", "foo", "baz2/foo", "bar", "baz2"}))
			Expect(notExisting).To(Equal([]string{"notexisting"}))
		})

		It("orders correctly when there are folders with folders", func() {
			testDir, err := os.MkdirTemp(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			err = os.MkdirAll(filepath.Join(testDir, "bar"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = os.MkdirAll(filepath.Join(testDir, "foo"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "foo", "bar"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(filepath.Join(testDir, "foo", "baz"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = os.MkdirAll(filepath.Join(testDir, "foo", "baz", "fa"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			ordered, _ := fileHelper.OrderFiles(testDir, []string{"foo", "foo/bar", "bar", "foo/baz/fa", "foo/baz"})
			Expect(ordered).To(Equal([]string{"foo/baz/fa", "foo/bar", "foo/baz", "foo", "bar"}))
		})
	})
})
