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

package client

import (
	"os"
	"path"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/pkg/errors"
)

type LocalClient struct {
	RepoData RepoData
	Cache    *artifact.ArtifactCache
	context  *types.Context
}

func NewLocalClient(r RepoData, ctx *types.Context) *LocalClient {
	return &LocalClient{
		Cache:    artifact.NewCache(ctx.Config.GetSystem().GetSystemPkgsCacheDirPath()),
		RepoData: r,
		context:  ctx,
	}
}

func (c *LocalClient) DownloadArtifact(a *artifact.PackageArtifact) (*artifact.PackageArtifact, error) {
	var err error

	artifactName := path.Base(a.Path)
	newart := a.ShallowCopy()

	fileName, err := c.Cache.Get(a)
	// Check if file is already in cache
	if err == nil {
		newart.Path = fileName
		c.context.Debug("Use artifact", artifactName, "from cache.")
	} else {
		d, err := c.DownloadFile(artifactName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed downloading %s", artifactName)
		}

		newart.Path = d
		c.Cache.Put(newart)
	}

	return newart, nil
}

func (c *LocalClient) DownloadFile(name string) (string, error) {
	var err error
	var file *os.File = nil

	rootfs := ""

	if !c.context.Config.ConfigFromHost {
		rootfs, err = c.context.Config.GetSystem().GetRootFsAbs()
		if err != nil {
			return "", err
		}
	}

	ok := false
	for _, uri := range c.RepoData.Urls {

		uri = filepath.Join(rootfs, uri)

		c.context.Info("Copying file", name, "from", uri)
		file, err = c.context.Config.GetSystem().TempFile("localclient")
		if err != nil {
			continue
		}
		//defer os.Remove(file.Name())

		err = fileHelper.CopyFile(filepath.Join(uri, name), file.Name())
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
