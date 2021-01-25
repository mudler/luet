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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	copy "github.com/otiai10/copy"
	"github.com/pkg/errors"
)

func OrderFiles(target string, files []string) ([]string, []string) {

	var newFiles []string
	var notPresent []string

	for _, f := range files {
		target := filepath.Join(target, f)
		fi, err := os.Lstat(target)
		if err != nil {
			notPresent = append(notPresent, f)
			continue
		}
		if m := fi.Mode(); !m.IsDir() {
			newFiles = append(newFiles, f)
		}
	}

	dirs := []string{}

	for _, f := range files {
		target := filepath.Join(target, f)
		fi, err := os.Lstat(target)
		if err != nil {
			continue
		}
		if m := fi.Mode(); m.IsDir() {
			dirs = append(dirs, f)
		}
	}

	// Compare how many sub paths there are, and push at the end the ones that have less subpaths
	sort.Slice(dirs, func(i, j int) bool {
		return len(strings.Split(dirs[i], string(os.PathSeparator))) > len(strings.Split(dirs[j], string(os.PathSeparator)))
	})

	return append(newFiles, dirs...), notPresent
}

func ListDir(dir string) ([]string, error) {
	content := []string{}

	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			content = append(content, path)

			return nil
		})

	return content, err
}

// Touch creates an empty file
func Touch(f string) error {
	_, err := os.Stat(f)
	if os.IsNotExist(err) {
		file, err := os.Create(f)
		if err != nil {
			return err
		}
		defer file.Close()
	} else {
		currentTime := time.Now().Local()
		err = os.Chtimes(f, currentTime, currentTime)
		if err != nil {
			return err
		}
	}
	return nil
}

// Exists reports whether the named file or directory exists.
func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func Read(file string) (string, error) {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func EnsureDir(fileName string) error {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		merr := os.MkdirAll(dirName, os.ModePerm) // FIXME: It should preserve permissions from src to dst instead
		if merr != nil {
			return merr
		}
	}
	return nil
}

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func CopyFile(src, dst string) (err error) {
	// Workaround for https://github.com/otiai10/copy/issues/47
	fi, err := os.Lstat(src)
	if err != nil {
		return errors.Wrap(err, "error reading file info")
	}

	fm := fi.Mode()
	switch {
	case fm&os.ModeNamedPipe != 0:
		EnsureDir(dst)
		if err := syscall.Mkfifo(dst, uint32(fi.Mode())); err != nil {
			return errors.Wrap(err, "failed creating pipe")
		}
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			if err := os.Chown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
				return errors.Wrap(err, "failed chowning file")
			}
		}
		return nil
	}

	return copy.Copy(src, dst, copy.Options{
		Sync:      true,
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow }})
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return copy.Copy(src, dst, copy.Options{
		Sync:      true,
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow }})
}
