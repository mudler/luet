// Copyright © 2020-2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-units"
	"github.com/pkg/errors"

	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/mudler/luet/pkg/helpers"

	"github.com/mudler/luet/pkg/helpers/docker"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
)

const (
	errImageDownloadMsg = "failed downloading image %s: %s"
)

type DockerClient struct {
	RepoData RepoData
	auth     *types.AuthConfig
	Cache    *artifact.ArtifactCache
	context  *luetTypes.Context
}

func NewDockerClient(r RepoData, ctx *luetTypes.Context) *DockerClient {
	auth := &types.AuthConfig{}

	dat, _ := json.Marshal(r.Authentication)
	json.Unmarshal(dat, auth)

	return &DockerClient{RepoData: r, auth: auth,
		Cache:   artifact.NewCache(ctx.Config.GetSystem().GetSystemPkgsCacheDirPath()),
		context: ctx,
	}
}

func (c *DockerClient) DownloadArtifact(a *artifact.PackageArtifact) (*artifact.PackageArtifact, error) {
	//var u *url.URL = nil
	var err error

	c.context.Spinner()
	defer c.context.SpinnerStop()

	resultingArtifact := a.ShallowCopy()
	artifactName := path.Base(a.Path)

	downloaded := false

	// TODO:
	// Files are in URI/packagename:version (GetPackageImageName() method)
	// use downloadAndExtract .. and egenrate an archive to consume. Checksum should be already checked while downloading the image
	// with the above functions, because Docker images already contain such metadata
	// - Check how verification is done when calling DownloadArtifact outside, similarly we need to check DownloadFile, and how verification
	// is done in such cases (see repository.go)

	// We discard checksum, that are checked while during pull and unpack by containerd
	resultingArtifact.Checksums = resultingArtifact.Checksums.Only(artifact.MTREE)

	// Check if file is already in cache
	fileName, err := c.Cache.Get(resultingArtifact)
	// Check if file is already in cache
	if err == nil {
		resultingArtifact = a.ShallowCopy()
		resultingArtifact.Path = fileName
		resultingArtifact.Checksums = resultingArtifact.Checksums.Only(artifact.MTREE)
		c.context.Debug("Use artifact", artifactName, "from cache.")
	} else {

		temp, err := c.context.Config.GetSystem().TempDir("image")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temp)

		tempArtifact, err := c.context.Config.GetSystem().TempFile("artifact")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tempArtifact.Name())
		for _, uri := range c.RepoData.Urls {

			imageName := fmt.Sprintf("%s:%s", uri, a.CompileSpec.GetPackage().ImageID())
			c.context.Info("Downloading image", imageName)

			// imageName := fmt.Sprintf("%s/%s", uri, artifact.GetCompileSpec().GetPackage().GetPackageImageName())
			info, err := docker.DownloadAndExtractDockerImage(c.context, imageName, temp, c.auth, c.RepoData.Verify)
			if err != nil {
				c.context.Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
				continue
			}

			c.context.Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
			c.context.Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.Target.Size))))
			c.context.Debug("\nCompressing result ", filepath.Join(temp), "to", tempArtifact.Name())

			resultingArtifact.Path = tempArtifact.Name() // First set to cache file
			err = resultingArtifact.Compress(temp, 1)
			if err != nil {
				c.context.Error(fmt.Sprintf("Failed compressing package %s: %s", imageName, err.Error()))
				continue
			}

			_, _, err = c.Cache.Put(resultingArtifact)
			if err != nil {
				c.context.Error(fmt.Sprintf("Failed storing package %s from cache: %s", imageName, err.Error()))
				continue
			}

			fileName, err := c.Cache.Get(resultingArtifact)
			if err != nil {
				c.context.Error(fmt.Sprintf("Failed getting package %s from cache: %s", imageName, err.Error()))
				continue
			}

			resultingArtifact.Path = fileName // Cache is persistent. tempArtifact is not

			downloaded = true
			break
		}

		if !downloaded {
			return nil, errors.Wrap(err, "no image available from repositories")
		}
	}

	return resultingArtifact, nil
}

func (c *DockerClient) DownloadFile(name string) (string, error) {
	var file *os.File = nil
	var err error
	var temp string
	// Files should be in URI/repository:<file>
	ok := false

	temp, err = c.context.Config.GetSystem().TempDir("tree")
	if err != nil {
		return "", err
	}

	for _, uri := range c.RepoData.Urls {
		file, err = c.context.Config.GetSystem().TempFile("DockerClient")
		if err != nil {
			continue
		}

		imageName := fmt.Sprintf("%s:%s", uri, helpers.SanitizeImageString(name))
		c.context.Info("Downloading", imageName)

		info, err := docker.DownloadAndExtractDockerImage(c.context, imageName, temp, c.auth, c.RepoData.Verify)
		if err != nil {
			c.context.Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
			continue
		}

		c.context.Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
		c.context.Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.Target.Size))))

		c.context.Debug("\nCopying file ", filepath.Join(temp, name), "to", file.Name())
		err = fileHelper.CopyFile(filepath.Join(temp, name), file.Name())
		if err != nil {
			continue
		}
		ok = true
		break
	}

	if !ok {
		return "", err
	}

	return file.Name(), err
}
