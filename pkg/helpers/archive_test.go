// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>
//                       Daniele Rondina <geaaru@sabayonlinux.org>
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

package helpers_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	. "github.com/mudler/luet/pkg/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Code from moby/moby pkg/archive/archive_test
func prepareUntarSourceDirectory(numberOfFiles int, targetPath string, makeLinks bool) (int, error) {
	fileData := []byte("fooo")
	for n := 0; n < numberOfFiles; n++ {
		fileName := fmt.Sprintf("file-%d", n)
		if err := ioutil.WriteFile(filepath.Join(targetPath, fileName), fileData, 0700); err != nil {
			return 0, err
		}
		if makeLinks {
			if err := os.Link(filepath.Join(targetPath, fileName), filepath.Join(targetPath, fileName+"-link")); err != nil {
				return 0, err
			}
		}
	}
	totalSize := numberOfFiles * len(fileData)
	return totalSize, nil
}

func tarModifierWrapperFunc(dst, path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
	// If the destination path already exists I rename target file name with postfix.
	var basePath string

	// Read data. TODO: We need change archive callback to permit to return a Reader
	buffer := bytes.Buffer{}
	if content != nil {
		if _, err := buffer.ReadFrom(content); err != nil {
			return nil, nil, err
		}
	}

	if header != nil {

		switch header.Typeflag {
		case tar.TypeReg:
			basePath = filepath.Base(path)
		default:
			// Nothing to do. I return original reader
			return header, buffer.Bytes(), nil
		}

		if basePath == "file-0" {
			name := filepath.Join(filepath.Join(filepath.Dir(path), fmt.Sprintf("._cfg%04d_%s", 1, basePath)))
			return &tar.Header{
				Mode:       header.Mode,
				Typeflag:   header.Typeflag,
				PAXRecords: header.PAXRecords,
				Name:       name,
			}, buffer.Bytes(), nil
		} else if basePath == "file-1" {
			return header, []byte("newcontent"), nil
		}

		// else file not present
	}

	return header, buffer.Bytes(), nil
}

var _ = Describe("Helpers Archive", func() {
	Context("Untar Protect", func() {

		It("Detect existing and not-existing files", func() {

			archiveSourceDir, err := ioutil.TempDir("", "archive-source")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(archiveSourceDir)

			_, err = prepareUntarSourceDirectory(10, archiveSourceDir, false)
			Expect(err).ToNot(HaveOccurred())

			targetDir, err := ioutil.TempDir("", "archive-target")
			Expect(err).ToNot(HaveOccurred())
			//	defer os.RemoveAll(targetDir)

			sourceArchive, err := archive.TarWithOptions(archiveSourceDir, &archive.TarOptions{})
			Expect(err).ToNot(HaveOccurred())
			defer sourceArchive.Close()

			tarModifier := NewTarModifierWrapper(targetDir, tarModifierWrapperFunc)
			mods := make(map[string]archive.TarModifierFunc)
			mods["file-0"] = tarModifier.GetModifier()
			mods["file-1"] = tarModifier.GetModifier()
			mods["file-9999"] = tarModifier.GetModifier()

			replacerArchive := archive.ReplaceFileTarWrapper(sourceArchive, mods)
			//replacerArchive := archive.ReplaceFileTarWrapper(sourceArchive, mods)
			opts := &archive.TarOptions{
				// NOTE: NoLchown boolean is used for chmod of the symlink
				// Probably it's needed set this always to true.
				NoLchown:        true,
				ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
				ContinueOnError: true,
			}

			err = archive.Untar(replacerArchive, targetDir, opts)
			Expect(err).ToNot(HaveOccurred())

			Expect(Exists(filepath.Join(targetDir, "._cfg0001_file-0"))).Should(Equal(true))
		})
	})
})
