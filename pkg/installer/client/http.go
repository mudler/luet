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
	"net/url"
	"os"
	"path"
	"path/filepath"

	. "github.com/mudler/luet/pkg/logger"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/helpers"

	"github.com/cavaliercoder/grab"
)

type HttpClient struct {
	RepoData RepoData
}

func NewHttpClient(r RepoData) *HttpClient {
	return &HttpClient{RepoData: r}
}

func (c *HttpClient) DownloadArtifact(artifact compiler.Artifact) (compiler.Artifact, error) {
	artifactName := path.Base(artifact.GetPath())
	Info("Downloading artifact", artifactName, "from", c.RepoData.Uri)

	temp, err := ioutil.TempDir(os.TempDir(), "tree")
	if err != nil {
		return nil, err
	}

	file, err := ioutil.TempFile(temp, "HttpClient")
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(c.RepoData.Uri)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, artifactName)

	_, err = grab.Get(temp, u.String())
	if err != nil {
		return nil, err
	}

	err = helpers.CopyFile(filepath.Join(temp, artifactName), file.Name())

	return compiler.NewPackageArtifact(file.Name()), nil
}

func (c *HttpClient) DownloadFile(name string) (string, error) {
	temp, err := ioutil.TempDir(os.TempDir(), "tree")
	if err != nil {
		return "", err
	}
	file, err := ioutil.TempFile(os.TempDir(), "HttpClient")
	if err != nil {
		return "", err
	}
	//defer os.Remove(file.Name())
	u, err := url.Parse(c.RepoData.Uri)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, name)

	Info("Downloading", u.String())

	_, err = grab.Get(temp, u.String())
	if err != nil {
		return "", err
	}

	err = helpers.CopyFile(filepath.Join(temp, name), file.Name())

	return file.Name(), err
}
