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
	"path/filepath"
	"strings"

	log "github.com/geaaru/tar-formers/pkg/logger"
	specs "github.com/geaaru/tar-formers/pkg/specs"
)

type TarFormers struct {
	Config *specs.Config `yaml:"config" json:"config"`
	Logger *log.Logger   `yaml:"-" json:"-"`

	reader io.Reader `yaml:"-" json:"-"`

	Task      *specs.SpecFile `yaml:"task,omitempty" json:"task,omitempty"`
	ExportDir string          `yaml:"export_dir,omitempty" json:"export_dir,omitempty"`
}

func NewTarFormers(config *specs.Config) *TarFormers {
	ans := &TarFormers{
		Config:    config,
		Logger:    log.NewLogger(config),
		Task:      nil,
		ExportDir: "",
	}

	// Initialize logging
	if config.GetLogging().EnableLogFile && config.GetLogging().Path != "" {
		err := ans.Logger.InitLogger2File()
		if err != nil {
			ans.Logger.Fatal("Error on initialize logfile")
		}
	}
	ans.Logger.SetAsDefault()
	return ans
}

func (t *TarFormers) SetReader(reader io.Reader) {
	t.reader = reader
}

func (t *TarFormers) RunTask(task *specs.SpecFile, dir string) error {
	if task == nil {
		return errors.New("Invalid task")
	}

	if dir == "" {
		return errors.New("Invalid export dir")
	}

	t.Task = task

	err := t.CreateDir(dir, 0755)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(t.reader)

	err = t.HandleTarFlow(tarReader, dir)
	if err != nil {
		return err
	}

	return nil
}

func (t *TarFormers) HandleTarFlow(tarReader *tar.Reader, dir string) error {
	var ans error = nil
	links := []specs.Link{}

	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			err = nil
			break
		}

		if err != nil {
			ans = err
			break
		}

		absPath := "/" + header.Name
		if t.Task.IsPath2Skip(absPath) {
			t.Logger.Debug(fmt.Sprintf("File %s skipped.", header.Name))
			continue
		}

		t.Logger.Debug(fmt.Sprintf("Parsing file %s [%s - %d, %s - %d] (%s).",
			header.Name, header.Uname, header.Uid, header.Gname, header.Gid, header.Linkname))

		targetPath := filepath.Join(dir, header.Name)
		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			err := t.CreateDir(targetPath, info.Mode())
			if err != nil {
				return errors.New(
					fmt.Sprintf("Error on create directory %s: %s",
						targetPath, err.Error()))
			}
		case tar.TypeReg, tar.TypeRegA:
			err := t.CreateFile(dir, info.Mode(), tarReader, header)
			if err != nil {
				return err
			}
		case tar.TypeLink:
			t.Logger.Debug(fmt.Sprintf("Path %s is a hardlink to %s.",
				header.Name, header.Linkname))
			links = append(links,
				specs.Link{
					Path:     targetPath,
					Linkname: filepath.Join(dir, header.Linkname),
					Name:     header.Name,
					Mode:     info.Mode(),
					TypeFlag: header.Typeflag,
				})
		case tar.TypeSymlink:
			t.Logger.Debug(fmt.Sprintf("Path %s is a symlink to %s.",
				header.Name, header.Linkname))
			links = append(links,
				specs.Link{
					Path:     targetPath,
					Linkname: header.Linkname,
					Name:     header.Name,
					Mode:     info.Mode(),
					TypeFlag: header.Typeflag,
				})
		case tar.TypeChar, tar.TypeBlock:
			err := t.CreateBlockCharFifo(targetPath, info.Mode(), header)
			if err != nil {
				return err
			}

		}

		// Set this an option
		switch header.Typeflag {
		case tar.TypeDir, tar.TypeReg, tar.TypeRegA, tar.TypeBlock, tar.TypeFifo:
			err := t.SetFileProps(targetPath, header)
			if err != nil {
				return err
			}
		}

	}

	// Create all links
	if len(links) > 0 {
		//links = t.GetOrderedLinks(links)
		for i := range links {
			err := t.CreateLink(links[i])
			if err != nil {
				return err
			}
		}
	}

	return ans
}
