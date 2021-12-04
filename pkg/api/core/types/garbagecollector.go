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

package types

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/pkg/errors"
)

// GarbageCollector keeps track of directory assigned and cleans them up
type GarbageCollector struct {
	dir          string
	createdDirs  []string
	createdFiles []string
}

// NewGC returns a new GC instance on dir
func NewGC(s string) (*GarbageCollector, error) {
	if !filepath.IsAbs(s) {
		abs, err := fileHelper.Rel2Abs(s)
		if err != nil {
			return nil, errors.Wrap(err, "while converting relative path to absolute path")
		}
		s = abs
	}
	if _, err := os.Stat(s); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(s, os.ModePerm)
			if err != nil {
				return nil, err
			}
		}
	}
	return &GarbageCollector{dir: s}, nil
}

func (gc *GarbageCollector) Directory(pattern string) (string, error) {
	dir, err := ioutil.TempDir(gc.dir, pattern)
	if err != nil {
		return "", err
	}
	gc.createdDirs = append(gc.createdDirs, dir)
	return dir, err
}

func (gc *GarbageCollector) Remove() error {
	return os.RemoveAll(gc.dir)
}

func (gc *GarbageCollector) Clean() (err error) {
	for _, d := range append(gc.createdDirs, gc.createdFiles...) {
		if fileHelper.Exists(d) {
			multierror.Append(err, os.RemoveAll(d))
		}
	}

	return
}

func (gc *GarbageCollector) File(pattern string) (*os.File, error) {
	f, err := ioutil.TempFile(gc.dir, pattern)
	if err != nil {
		return nil, err
	}
	gc.createdFiles = append(gc.createdFiles, f.Name())
	return f, err
}
