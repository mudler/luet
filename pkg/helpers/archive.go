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

package helpers

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
)

func Tar(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	fs, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = io.Copy(out, fs)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}
	return err
}

type TarModifierWrapperFunc func(path, dst string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error)
type TarModifierWrapper struct {
	DestinationPath string
	Modifier        TarModifierWrapperFunc
}

func NewTarModifierWrapper(dst string, modifier TarModifierWrapperFunc) *TarModifierWrapper {
	return &TarModifierWrapper{
		DestinationPath: dst,
		Modifier:        modifier,
	}
}

func (m *TarModifierWrapper) GetModifier() archive.TarModifierFunc {
	return func(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
		return m.Modifier(m.DestinationPath, path, header, content)
	}
}

func UntarProtect(src, dst string, sameOwner bool, protectedFiles []string, modifier *TarModifierWrapper) error {
	var ans error

	if len(protectedFiles) <= 0 {
		return Untar(src, dst, sameOwner)
	}

	// POST: we have files to protect. I create a ReplaceFileTarWrapper
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Create modifier map
	mods := make(map[string]archive.TarModifierFunc)
	for _, file := range protectedFiles {
		mods[file] = modifier.GetModifier()
	}

	if sameOwner {
		// we do have root permissions, so we can extract keeping the same permissions.
		replacerArchive := archive.ReplaceFileTarWrapper(in, mods)

		opts := &archive.TarOptions{
			NoLchown:        false,
			ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
			ContinueOnError: true,
		}

		ans = archive.Untar(replacerArchive, dst, opts)
	} else {
		ans = unTarIgnoreOwner(dst, in, mods)
	}

	return ans
}

func unTarIgnoreOwner(dest string, in io.ReadCloser, mods map[string]archive.TarModifierFunc) error {
	tr := tar.NewReader(in)
	for {
		header, err := tr.Next()

		var data []byte
		var headerReplaced = false

		switch {
		case err == io.EOF:
			goto tarEof
		case err != nil:
			return err
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dest, header.Name)
		if mods != nil {
			modifier, ok := mods[header.Name]
			if ok {
				header, data, err = modifier(header.Name, header, tr)
				if err != nil {
					return err
				}

				// Override target path
				target = filepath.Join(dest, header.Name)
				headerReplaced = true
			}

		}

		// Check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

			// handle creation of file
		case tar.TypeReg:

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if headerReplaced {
				_, err = io.Copy(f, bytes.NewReader(data))
			} else {
				_, err = io.Copy(f, tr)
			}
			if err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each
			// file close to wait until all operations have completed.
			f.Close()

		case tar.TypeSymlink:
			source := header.Linkname
			err := os.Symlink(source, target)
			if err != nil {
				return err
			}
		}
	}
tarEof:

	return nil
}

// Untar just a wrapper around the docker functions
func Untar(src, dest string, sameOwner bool) error {
	var ans error

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if sameOwner {
		opts := &archive.TarOptions{
			NoLchown:        false,
			ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
			ContinueOnError: true,
		}

		ans = archive.Untar(in, dest, opts)
	} else {
		ans = unTarIgnoreOwner(dest, in, nil)
	}

	return ans
}
