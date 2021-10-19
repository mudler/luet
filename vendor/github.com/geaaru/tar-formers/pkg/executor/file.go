/*

Copyright (C) 2021  Daniele Rondina <geaaru@sabayonlinux.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/
package executor

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	specs "github.com/geaaru/tar-formers/pkg/specs"

	"golang.org/x/sys/unix"
)

func (t *TarFormers) CreateFile(dir string, mode os.FileMode, reader *tar.Reader, header *tar.Header) error {

	file := t.Task.GetRename("/" + header.Name)
	file = filepath.Join(dir, file)

	err := t.CreateDir(filepath.Dir(file), mode|os.ModeDir|100)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return errors.New(
			fmt.Sprintf("Error on open file %s: %s", file, err.Error()))
	}
	defer f.Close()

	// Copy file content
	nb, err := io.Copy(f, reader)
	if err != nil {
		return errors.New(
			fmt.Sprintf("Error on write file %s: %s", file, err.Error()))
	}
	if nb != header.Size {
		return errors.New(
			fmt.Sprintf("For file %s written file are different %d - %d",
				file, nb, header.Size))
	}

	t.Logger.Debug(fmt.Sprintf(
		"Created file %s (size %d).", file, nb))

	// TODO: check if it's needed f.Sync()
	//if err := f.Sync(); err != nil {
	//	return err
	//}

	return nil
}

func (t *TarFormers) ExistFile(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func (t *TarFormers) SetFileProps(path string, header *tar.Header) error {
	if t.Task.SameOwner {
		if err := os.Chown(path, header.Uid, header.Gid); err != nil {
			return errors.New(
				fmt.Sprintf("For path %s error on chown: %s",
					path, err.Error()))
		}
	}

	// maintaining access and modification time in best effort fashion
	if t.Task.SameChtimes {
		err := os.Chtimes(path, header.AccessTime, header.ModTime)
		if err != nil {
			t.Logger.Warning(
				"[%s] Error on chtimes: %s", path, err.Error())
		}
	}

	return nil
}

func (t *TarFormers) CreateBlockCharFifo(file string, mode os.FileMode, header *tar.Header) error {
	err := t.CreateDir(filepath.Dir(file), mode|os.ModeDir|100)
	if err != nil {
		return err
	}

	modeDev := uint32(header.Mode & 07777)
	switch header.Typeflag {
	case tar.TypeBlock:
		modeDev |= unix.S_IFBLK
	case tar.TypeChar:
		modeDev |= unix.S_IFCHR
	case tar.TypeFifo:
		modeDev |= unix.S_IFIFO
	}

	dev := int(uint32(unix.Mkdev(uint32(header.Devmajor), uint32(header.Devminor))))
	return unix.Mknod(file, modeDev, dev)
}

func (t *TarFormers) CreateLink(link specs.Link) error {

	if link.TypeFlag == tar.TypeSymlink {
		t.Logger.Debug("Creating symlink ", link.Name, link.Path)
		if err := syscall.Symlink(link.Linkname, link.Path); err != nil {
			errmsg := fmt.Sprintf(
				"Error on create symlink %s -> %s (%s): %s",
				link.Path, link.Linkname, link.Name, err.Error())

			if t.Task.BrokenLinksFatal {
				return errors.New(errmsg)
			} else {
				t.Logger.Warning("WARNING: " + errmsg)
			}
		}
	} else {
		if err := syscall.Link(link.Linkname, link.Path); err != nil {
			errmsg := fmt.Sprintf(
				"Error on create hardlink %s -> %s (%s): %s",
				link.Path, link.Linkname, link.Name, err.Error())

			if t.Task.BrokenLinksFatal {
				return errors.New(errmsg)
			} else {
				t.Logger.Warning("WARNING: " + errmsg)
			}
		}
	}

	return nil
}
