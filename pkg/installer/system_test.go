// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package installer_test

import (

	//	. "github.com/mudler/luet/pkg/installer"

	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	. "github.com/mudler/luet/pkg/installer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("System", func() {
	Context("Files", func() {
		var s *System
		var db types.PackageDatabase
		var a, b *types.Package
		ctx := context.NewContext()

		BeforeEach(func() {
			db = pkg.NewInMemoryDatabase(false)
			s = &System{Database: db}

			a = &types.Package{Name: "test", Version: "1", Category: "t"}

			db.CreatePackage(a)
			db.SetPackageFiles(&types.PackageFile{PackageFingerprint: a.GetFingerPrint(), Files: []string{"foo", "f"}})

			b = &types.Package{Name: "test2", Version: "1", Category: "t"}

			db.CreatePackage(b)
			db.SetPackageFiles(&types.PackageFile{PackageFingerprint: b.GetFingerPrint(), Files: []string{"barz", "f"}})
		})

		It("detects when are already shipped by other packages", func() {
			r, p, err := s.ExistsPackageFile("foo")
			Expect(r).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(p).To(Equal(a))
			r, p, err = s.ExistsPackageFile("baz")
			Expect(r).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
			Expect(p).To(BeNil())

			r, p, err = s.ExistsPackageFile("f")
			Expect(r).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(p).To(Or(Equal(b), Equal(a))) // This fails
			r, p, err = s.ExistsPackageFile("barz")
			Expect(r).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(p).To(Equal(b))
		})

		It("detect missing files", func() {
			dir, err := ioutil.TempDir("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dir)
			s.Target = dir
			notfound := s.OSCheck(ctx)
			Expect(len(notfound)).To(Equal(2))
			ioutil.WriteFile(filepath.Join(dir, "f"), []byte{}, os.ModePerm)
			ioutil.WriteFile(filepath.Join(dir, "foo"), []byte{}, os.ModePerm)
			notfound = s.OSCheck(ctx)
			Expect(len(notfound)).To(Equal(1))
		})
	})
})
