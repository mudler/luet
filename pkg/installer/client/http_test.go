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

package client_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	compiler "github.com/mudler/luet/pkg/compiler"
	helpers "github.com/mudler/luet/pkg/helpers"

	. "github.com/mudler/luet/pkg/installer/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Http client", func() {
	Context("With repository", func() {

		It("Downloads single files", func() {
			// setup small staticfile webserver with content
			tmpdir, err := ioutil.TempDir("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up
			Expect(err).ToNot(HaveOccurred())
			ts := httptest.NewServer(http.FileServer(http.Dir(tmpdir)))
			defer ts.Close()
			err = ioutil.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(`test`), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			c := NewHttpClient(RepoData{Urls: []string{ts.URL}})
			path, err := c.DownloadFile("test.txt")
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Read(path)).To(Equal("test"))
			os.RemoveAll(path)
		})

		It("Downloads artifacts", func() {
			// setup small staticfile webserver with content
			tmpdir, err := ioutil.TempDir("", "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir) // clean up
			Expect(err).ToNot(HaveOccurred())
			ts := httptest.NewServer(http.FileServer(http.Dir(tmpdir)))
			defer ts.Close()
			err = ioutil.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(`test`), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			c := NewHttpClient(RepoData{Urls: []string{ts.URL}})
			path, err := c.DownloadArtifact(&compiler.PackageArtifact{Path: "test.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(helpers.Read(path.GetPath())).To(Equal("test"))
			os.RemoveAll(path.GetPath())
		})

	})
})
