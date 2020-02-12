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
	"path"
	"path/filepath"

	. "github.com/mudler/luet/pkg/logger"
	"github.com/mudler/luet/pkg/config"

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
	var err error

	artifactName := path.Base(artifact.GetPath())
	cacheFile := filepath.Join(config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), artifactName)

	// Check if file is already in cache
	if helpers.Exists(cacheFile) {
		Info("Use artifact", artifactName, "from cache.")
	} else {
		ok := false
		for _, uri := range c.RepoData.Urls {
			Info("Downloading artifact", artifactName, "from", uri)

			//defer os.Remove(file.Name())
			err = helpers.CopyFile(filepath.Join(uri, artifactName), cacheFile)
			if err != nil {
				continue
			}
			ok = true
			break
		}

		if !ok {
			return nil, err
		}
	}

	newart := artifact
	newart.SetPath(cacheFile)
	return newart, nil
}

func (c *LocalClient) DownloadFile(name string) (string, error) {
	var err error
	var file *os.File = nil

	ok := false
	for _, uri := range c.RepoData.Urls {
		Info("Downloading file", name, "from", uri)
		file, err = ioutil.TempFile(os.TempDir(), "localclient")
		if err != nil {
			continue
		}
		//defer os.Remove(file.Name())

		err = helpers.CopyFile(filepath.Join(uri, name), file.Name())
		if err != nil {
			continue
		}
		ok = true
		break
	}

	if ok {
		return file.Name(), nil
	}

	return "", err
}
