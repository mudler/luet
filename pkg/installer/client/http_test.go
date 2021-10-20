// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/mudler/luet/pkg/installer/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Http client", func() {
	Context("With repository", func() {
		ctx := types.NewContext()

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

			c := NewHttpClient(RepoData{Urls: []string{ts.URL}}, ctx)
			path, err := c.DownloadFile("test.txt")
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Read(path)).To(Equal("test"))
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

			c := NewHttpClient(RepoData{Urls: []string{ts.URL}}, ctx)
			path, err := c.DownloadArtifact(&artifact.PackageArtifact{Path: "test.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(fileHelper.Read(path.Path)).To(Equal("test"))
			os.RemoveAll(path.Path)
		})

	})
})
