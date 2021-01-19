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
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-units"
	"github.com/moby/buildkit/session"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/genuinetools/img/client"

	"github.com/genuinetools/img/types"
	"github.com/moby/buildkit/util/appcontext"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
)

type DockerClient struct {
	RepoData RepoData
}

func NewDockerClient(r RepoData) *DockerClient {
	return &DockerClient{RepoData: r}
}

func downloadAndExtractDockerImage(image, dest string) error {
	temp, err := config.LuetCfg.GetSystem().TempDir("contentstore")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	Debug("Temporary directory", temp)
	c, err := client.New(temp, types.NativeBackend, nil)
	if err != nil {
		return errors.Wrapf(err, "failed creating client")
	}
	defer c.Close()

	// Slightly adapted from https://github.com/genuinetools/img/blob/54d0ca981c1260546d43961a538550eef55c87cf/pull.go
	var listedImage *client.ListedImage
	// Create the context.
	ctx := appcontext.Context()
	sess, sessDialer, err := c.Session(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed creating Session")
	}
	ctx = session.NewContext(ctx, sess.ID())
	ctx = namespaces.WithNamespace(ctx, "buildkit")

	Debug("Starting session")
	go func() {
		sess.Run(ctx, sessDialer)

	}()
	defer func() {
		Debug("Closing session")
		sess.Close()
		Debug("Session closed")
	}()

	Debug("Pulling image", image)
	listedImage, err = c.Pull(ctx, image)
	if err != nil {
		return errors.Wrapf(err, "failed listing images")

	}

	Debug("Pulled:", listedImage.Target.Digest)
	Debug("Size:", units.BytesSize(float64(listedImage.ContentSize)))
	Debug("Unpacking", image, "to", dest)
	os.RemoveAll(dest)

	// XXX: Unpacking stalls. See why calling img works, and with luet doesn't. Shall we unpack by reimplementing the client here?

	// err = c.Unpack(ctx, image, dest)
	// Debug("Finished Unpacking")

	// opt, err := c.createWorkerOpt(false)
	// if err != nil {
	// 	return fmt.Errorf("creating worker opt failed: %v", err)
	// }

	img, err := opt.ImageStore.Get(ctx, image)
	if err != nil {
		return fmt.Errorf("getting image %s from image store failed: %v", image, err)
	}

	manifest, err := images.Manifest(ctx, opt.ContentStore, img.Target, platforms.Default())
	if err != nil {
		return fmt.Errorf("getting image manifest failed: %v", err)
	}

	for _, desc := range manifest.Layers {
		logrus.Debugf("Unpacking layer %s", desc.Digest.String())

		// Read the blob from the content store.
		layer, err := opt.ContentStore.ReaderAt(ctx, desc)
		if err != nil {
			return fmt.Errorf("getting reader for digest %s failed: %v", desc.Digest.String(), err)
		}

		// Unpack the tarfile to the rootfs path.
		// FROM: https://godoc.org/github.com/moby/moby/pkg/archive#TarOptions
		if err := archive.Untar(content.NewReader(layer), dest, &archive.TarOptions{
			NoLchown: true,
		}); err != nil {
			return fmt.Errorf("extracting tar for %s to directory %s failed: %v", desc.Digest.String(), dest, err)
		}
	}

	return errors.Wrapf(err, "failed unpacking images")

	// eg, ctx := errgroup.WithContext(ctx)

	// eg.Go(func() error {
	// 	return sess.Run(ctx, sessDialer)
	// })
	// eg.Go(func() error {
	// 	defer sess.Close()
	// 	var err error
	// 	listedImage, err = c.Pull(ctx, image)
	// 	if err != nil {
	// 		return errors.Wrapf(err, "failed listing images")

	// 	}
	// 	os.RemoveAll(dest)
	// 	return errors.Wrapf(c.Unpack(ctx, image, dest), "failed unpacking images")
	// })

	// if err := eg.Wait(); err != nil {
	// 	return err
	// }

	//Debug("Pulled:", listedImage.Target.Digest)
	//	Debug("Size:", units.BytesSize(float64(listedImage.ContentSize)))

	// Get the identifier for the image.
	// id, err := source.NewImageIdentifier(image)
	// if err != nil {
	// 	return err
	// }
	// Debug("Image identifier", id.ID())

	// named, err := reference.ParseNormalizedNamed(image)
	// if err != nil {
	// 	return fmt.Errorf("parsing image name %q failed: %v", image, err)
	// }
	// // Add the latest lag if they did not provide one.
	// named = reference.TagNameOnly(named)
	// image = named.String()

	// ctx := appcontext.Context()
	// sess, sessDialer, err := c.Session(ctx)
	// if err != nil {
	// 	return err
	// }
	// ctx = session.NewContext(ctx, sess.ID())
	// ctx = namespaces.WithNamespace(ctx, "buildkit")

	// snapshotRoot := filepath.Join(temp, "snapshots")

	// XXX: We force native backend. Our docker images will have just one layer as they are created from scratch.
	// No need to depend on FUSE/overlayfs available in the system
	// s, err := native.NewSnapshotter(snapshotRoot)

	// contentStore, err := local.NewStore(filepath.Join(temp, "content"))
	// if err != nil {
	// 	return err
	// }

	// // Open the bolt database for metadata.
	// db, err := bolt.Open(filepath.Join(temp, "containerdmeta.db"), 0644, nil)
	// if err != nil {
	// 	return err
	// }

	// // Create the new database for metadata.
	// mdb := ctdmetadata.NewDB(db, contentStore, map[string]ctdsnapshot.Snapshotter{
	// 	types.NativeBackend: s,
	// })
	// if err := mdb.Init(ctx); err != nil {
	// 	return err
	// }

	// // Create the image store.
	// imageStore := ctdmetadata.NewImageStore(mdb)

	// contentStore = containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit")

	// Debug("Getting image", image)
	// img, err := imageStore.Get(ctx, image)
	// if err != nil {
	// 	return fmt.Errorf("getting image %s from image store failed: %v", image, err)
	// }

	// manifest, err := images.Manifest(ctx, contentStore, img.Target, platforms.Default())
	// if err != nil {
	// 	return fmt.Errorf("getting image manifest failed: %v", err)
	// }

	// for _, desc := range manifest.Layers {
	// 	Debug("Unpacking layer %s", desc.Digest.String())

	// 	// Read the blob from the content store.
	// 	layer, err := contentStore.ReaderAt(ctx, desc)
	// 	if err != nil {
	// 		return fmt.Errorf("getting reader for digest %s failed: %v", desc.Digest.String(), err)
	// 	}

	// 	// Unpack the tarfile to the rootfs path.
	// 	// FROM: https://godoc.org/github.com/moby/moby/pkg/archive#TarOptions
	// 	if err := archive.Untar(content.NewReader(layer), dest, &archive.TarOptions{
	// 		NoLchown:        true,
	// 		ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
	// 	}); err != nil {
	// 		return fmt.Errorf("extracting tar for %s to directory %s failed: %v", desc.Digest.String(), dest, err)
	// 	}
	// }
	return nil
}

func (c *DockerClient) DownloadArtifact(artifact compiler.Artifact) (compiler.Artifact, error) {
	//var u *url.URL = nil
	var err error
	var temp string
	var resultingArtifact compiler.Artifact
	artifactName := path.Base(artifact.GetPath())
	cacheFile := filepath.Join(config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), artifactName)
	ok := false

	// TODO:
	// Files are in URI/packagename:version (GetPackageImageName() method)
	// use downloadAndExtract .. and egenrate an archive to consume. Checksum should be already checked while downloading the image
	// with the above functions, because Docker images already contain such metadata
	// - Check how verification is done when calling DownloadArtifact outside, similarly we need to check DownloadFile, and how verification
	// is done in such cases (see repository.go)

	// Check if file is already in cache
	if helpers.Exists(cacheFile) {
		Info("Use artifact", artifactName, "from cache.")
	} else {

		temp, err = config.LuetCfg.GetSystem().TempDir("tree")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temp)

		for _, uri := range c.RepoData.Urls {
			Debug("Downloading artifact", artifactName, "from", uri)

			imageName := fmt.Sprintf("%s:%s", uri, artifact.GetCompileSpec().GetPackage().GetFingerPrint())

			// imageName := fmt.Sprintf("%s/%s", uri, artifact.GetCompileSpec().GetPackage().GetPackageImageName())
			err = downloadAndExtractDockerImage(imageName, temp)
			if err != nil {
				Debug("Failed download of image", imageName)
				continue
			}
			Debug("\nCompressing result ", filepath.Join(temp, artifactName), "to", cacheFile)

			// We discard checksum, that are checked while during pull and unpack
			newart := artifact
			newart.SetPath(cacheFile)
			err = newart.Compress(temp, 1)
			if err != nil {
				Debug("Failed compressing package", imageName)
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

		Debug("Downloading file", name, "from", uri)

		imageName := fmt.Sprintf("%s:%s", uri, name)
		//imageName := fmt.Sprintf("%s/%s:%s", uri, "repository", name)
		err = downloadAndExtractDockerImage(imageName, temp)
		if err != nil {
			Debug("Failed download of image", imageName)
			continue
		}

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
