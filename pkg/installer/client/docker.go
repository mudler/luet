// Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
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

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
)

const (
	errImageDownloadMsg = "failed downloading image %s: %s"
)

type DockerClient struct {
	RepoData RepoData
	auth     *types.AuthConfig
	verify   bool
}

func NewDockerClient(r RepoData) *DockerClient {
	auth := &types.AuthConfig{}

	dat, _ := json.Marshal(r.Authentication)
	json.Unmarshal(dat, auth)

	return &DockerClient{RepoData: r, auth: auth}
}

func (c *DockerClient) DownloadArtifact(artifact compiler.Artifact) (compiler.Artifact, error) {
	//var u *url.URL = nil
	var err error
	var temp string

	Spinner(22)
	defer SpinnerStop()

	var resultingArtifact compiler.Artifact
	artifactName := path.Base(artifact.GetPath())
	cacheFile := filepath.Join(config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), artifactName)
	Debug("Cache file", cacheFile)
	if err := helpers.EnsureDir(cacheFile); err != nil {
		return nil, errors.Wrapf(err, "could not create cache folder %s for %s", config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), cacheFile)
	}
	ok := false

	// TODO:
	// Files are in URI/packagename:version (GetPackageImageName() method)
	// use downloadAndExtract .. and egenrate an archive to consume. Checksum should be already checked while downloading the image
	// with the above functions, because Docker images already contain such metadata
	// - Check how verification is done when calling DownloadArtifact outside, similarly we need to check DownloadFile, and how verification
	// is done in such cases (see repository.go)

	// Check if file is already in cache
	if helpers.Exists(cacheFile) {
		Debug("Cache hit for artifact", artifactName)
		resultingArtifact = artifact
		resultingArtifact.SetPath(cacheFile)
		resultingArtifact.SetChecksums(compiler.Checksums{})
	} else {

		temp, err = config.LuetCfg.GetSystem().TempDir("tree")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temp)

		for _, uri := range c.RepoData.Urls {

			imageName := fmt.Sprintf("%s:%s", uri, artifact.GetCompileSpec().GetPackage().ImageID())
			Info("Downloading image", imageName)

			contentstore, err := config.LuetCfg.GetSystem().TempDir("contentstore")
			if err != nil {
				Warning("Cannot create contentstore", err.Error())
				continue
			}

			// imageName := fmt.Sprintf("%s/%s", uri, artifact.GetCompileSpec().GetPackage().GetPackageImageName())
			info, err := helpers.DownloadAndExtractDockerImage(contentstore, imageName, temp, c.auth, c.RepoData.Verify)
			if err != nil {
				Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
				continue
			}

			Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
			Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.ContentSize))))
			Debug("\nCompressing result ", filepath.Join(temp), "to", cacheFile)

			newart := artifact
			// We discard checksum, that are checked while during pull and unpack
			newart.SetChecksums(compiler.Checksums{})
			newart.SetPath(cacheFile)                    // First set to cache file
			newart.SetPath(newart.GetUncompressedName()) // Calculate the real path from cacheFile
			err = newart.Compress(temp, 1)
			if err != nil {
				Error(fmt.Sprintf("Failed compressing package %s: %s", imageName, err.Error()))
				continue
			}
			resultingArtifact = newart

			ok = true
			break
		}

		if !ok {
			return nil, err
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

	temp, err = config.LuetCfg.GetSystem().TempDir("tree")
	if err != nil {
		return "", err
	}

	for _, uri := range c.RepoData.Urls {

		file, err = config.LuetCfg.GetSystem().TempFile("DockerClient")
		if err != nil {
			continue
		}

		contentstore, err := config.LuetCfg.GetSystem().TempDir("contentstore")
		if err != nil {
			Warning("Cannot create contentstore", err.Error())
			continue
		}

		imageName := fmt.Sprintf("%s:%s", uri, name)
		Info("Downloading", imageName)

		info, err := helpers.DownloadAndExtractDockerImage(contentstore, imageName, temp, c.auth, c.RepoData.Verify)
		if err != nil {
			Warning(fmt.Sprintf(errImageDownloadMsg, imageName, err.Error()))
			continue
		}

		Info(fmt.Sprintf("Pulled: %s", info.Target.Digest))
		Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.ContentSize))))

		Debug("\nCopying file ", filepath.Join(temp, name), "to", file.Name())
		err = helpers.CopyFile(filepath.Join(temp, name), file.Name())

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
