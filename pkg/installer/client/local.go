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

package client

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/helpers"
)

type LocalClient struct {
	RepoData RepoData
}

func NewLocalClient(r RepoData) *LocalClient {
	return &LocalClient{RepoData: r}
}

func (c *LocalClient) DownloadArtifact(artifact compiler.Artifact) (compiler.Artifact, error) {

	file, err := ioutil.TempFile(os.TempDir(), "localclient")
	if err != nil {
		return nil, err
	}
	//defer os.Remove(file.Name())

	err = helpers.CopyFile(filepath.Join(c.RepoData.Uri, artifact.GetPath()), file.Name())

	return compiler.NewPackageArtifact(file.Name()), nil
}
func (c *LocalClient) DownloadFile(name string) (string, error) {

	file, err := ioutil.TempFile(os.TempDir(), "localclient")
	if err != nil {
		return "", err
	}
	//defer os.Remove(file.Name())

	err = helpers.CopyFile(filepath.Join(c.RepoData.Uri, name), file.Name())

	return file.Name(), err
}
