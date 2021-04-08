// Copyright © 2019-2021 Ettore Di Giacinto <mudler@sabayon.org>
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

package installer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/pkg/bus"
	compiler "github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	"github.com/pkg/errors"
)

type dockerRepositoryGenerator struct {
	b                compiler.CompilerBackend
	imagePrefix      string
	imagePush, force bool
}

func (l *dockerRepositoryGenerator) Initialize(path string, db pkg.PackageDatabase) ([]compiler.Artifact, error) {
	return generatePackageImages(l.b, l.imagePrefix, path, db, l.imagePush, l.force)
}

func pushImage(b compiler.CompilerBackend, image string, force bool) error {
	if b.ImageAvailable(image) && !force {
		Debug("Image", image, "already present, skipping")
		return nil
	}
	return b.Push(compiler.CompilerBackendOptions{ImageName: image})
}

func generatePackageImages(b compiler.CompilerBackend, imagePrefix, path string, db pkg.PackageDatabase, imagePush, force bool) ([]compiler.Artifact, error) {
	Info("Generating docker images for packages in", imagePrefix)
	var art []compiler.Artifact
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		if !strings.HasSuffix(info.Name(), ".metadata.yaml") {
			return nil // Skip with no errors
		}

		dat, err := ioutil.ReadFile(currentpath)
		if err != nil {
			return errors.Wrap(err, "Error reading file "+currentpath)
		}

		artifact, err := compiler.NewPackageArtifactFromYaml(dat)
		if err != nil {
			return errors.Wrap(err, "Error reading yaml "+currentpath)
		}
		// Set the path relative to the file.
		// The metadata contains the full path where the file was located during buildtime.
		artifact.SetPath(filepath.Join(filepath.Dir(currentpath), filepath.Base(artifact.GetPath())))

		// We want to include packages that are ONLY referenced in the tree.
		// the ones which aren't should be deleted. (TODO: by another cli command?)
		if _, notfound := db.FindPackage(artifact.GetCompileSpec().GetPackage()); notfound != nil {
			Debug(fmt.Sprintf("Package %s not found in tree. Ignoring it.",
				artifact.GetCompileSpec().GetPackage().HumanReadableString()))
			return nil
		}

		packageImage := fmt.Sprintf("%s:%s", imagePrefix, artifact.GetCompileSpec().GetPackage().ImageID())

		if imagePush && b.ImageAvailable(packageImage) && !force {
			Info("Image", packageImage, "already present, skipping. use --force-push to override")
		} else {
			Info("Generating final image", packageImage,
				"for package ", artifact.GetCompileSpec().GetPackage().HumanReadableString())
			if opts, err := artifact.GenerateFinalImage(packageImage, b, true); err != nil {
				return errors.Wrap(err, "Failed generating metadata tree"+opts.ImageName)
			}
		}
		if imagePush {
			if err := pushImage(b, packageImage, force); err != nil {
				return errors.Wrapf(err, "Failed while pushing image: '%s'", packageImage)
			}
		}

		art = append(art, artifact)

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return nil, err

	}
	return art, nil
}

func (d *dockerRepositoryGenerator) Generate(r *LuetSystemRepository, imagePrefix string, resetRevision bool) error {
	// - Iterate over meta, build final images, push them if necessary
	//   - while pushing, check if image already exists, and if exist push them only if --force is supplied
	// - Generate final images for metadata and push

	imageRepository := fmt.Sprintf("%s:%s", imagePrefix, REPOSITORY_SPECFILE)

	r.LastUpdate = strconv.FormatInt(time.Now().Unix(), 10)

	repoTemp, err := config.LuetCfg.GetSystem().TempDir("repo")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for repository")
	}
	defer os.RemoveAll(repoTemp) // clean up

	if r.GetBackend().ImageAvailable(imageRepository) {
		if err := r.GetBackend().DownloadImage(compiler.CompilerBackendOptions{ImageName: imageRepository}); err != nil {
			return errors.Wrapf(err, "while downloading '%s'", imageRepository)
		}

		if err := r.GetBackend().ExtractRootfs(compiler.CompilerBackendOptions{ImageName: imageRepository, Destination: repoTemp}, false); err != nil {
			return errors.Wrapf(err, "while extracting '%s'", imageRepository)
		}
	}

	repospec := filepath.Join(repoTemp, REPOSITORY_SPECFILE)
	if resetRevision {
		r.Revision = 0
	} else {
		if _, err := os.Stat(repospec); !os.IsNotExist(err) {
			// Read existing file for retrieve revision
			spec, err := r.ReadSpecFile(repospec, false)
			if err != nil {
				return err
			}
			r.Revision = spec.GetRevision()
		}
	}
	r.Revision++

	Info(fmt.Sprintf(
		"For repository %s creating revision %d and last update %s...",
		r.Name, r.Revision, r.LastUpdate,
	))

	bus.Manager.Publish(bus.EventRepositoryPreBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: imageRepository,
	})

	// Create tree and repository file
	archive, err := config.LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up
	err = r.GetTree().Save(archive)
	if err != nil {
		return errors.Wrap(err, "Error met while saving the tree")
	}

	treeFile, err := r.GetRepositoryFile(REPOFILE_TREE_KEY)
	if err != nil {
		treeFile = NewDefaultTreeRepositoryFile()
		r.SetRepositoryFile(REPOFILE_TREE_KEY, treeFile)
	}

	a := compiler.NewPackageArtifact(filepath.Join(repoTemp, treeFile.GetFileName()))
	a.SetCompressionType(treeFile.GetCompressionType())
	err = a.Compress(archive, 1)
	if err != nil {
		return errors.Wrap(err, "Error met while creating package archive")
	}

	// Update the tree name with the name created by compression selected.
	treeFile.SetFileName(a.GetFileName())
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for tree")
	}
	treeFile.SetChecksums(a.GetChecksums())
	r.SetRepositoryFile(REPOFILE_TREE_KEY, treeFile)

	// we generate a new archive containing the required compressed file.
	// TODO: Bundle all the extra files in 1 docker image only, instead of an image for each file
	treeArchive, err := compiler.CreateArtifactForFile(a.GetPath())
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for tree")
	}
	imageTree := fmt.Sprintf("%s:%s", imagePrefix, a.GetFileName())
	Debug("Generating image", imageTree)
	if opts, err := treeArchive.GenerateFinalImage(imageTree, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating metadata tree "+opts.ImageName)
	}
	if r.PushImages {
		if err := pushImage(r.GetBackend(), imageTree, true); err != nil {
			return errors.Wrapf(err, "Failed while pushing image: '%s'", imageTree)
		}
	}

	// Create Metadata struct and serialized repository
	meta, serialized := r.Serialize()

	// Create metadata file and repository file
	metaTmpDir, err := config.LuetCfg.GetSystem().TempDir("metadata")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for metadata")
	}
	defer os.RemoveAll(metaTmpDir) // clean up

	metaFile, err := r.GetRepositoryFile(REPOFILE_META_KEY)
	if err != nil {
		metaFile = NewDefaultMetaRepositoryFile()
		r.SetRepositoryFile(REPOFILE_META_KEY, metaFile)
	}

	repoMetaSpec := filepath.Join(metaTmpDir, REPOSITORY_METAFILE)
	// Create repository.meta.yaml file
	err = meta.WriteFile(repoMetaSpec)
	if err != nil {
		return err
	}

	// create temp dir for metafile
	metaDir, err := config.LuetCfg.GetSystem().TempDir("metadata")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for metadata")
	}
	defer os.RemoveAll(metaDir) // clean up

	a = compiler.NewPackageArtifact(filepath.Join(metaDir, metaFile.GetFileName()))
	a.SetCompressionType(metaFile.GetCompressionType())
	err = a.Compress(metaTmpDir, 1)
	if err != nil {
		return errors.Wrap(err, "Error met while archiving repository metadata")
	}

	metaFile.SetFileName(a.GetFileName())
	r.SetRepositoryFile(REPOFILE_META_KEY, metaFile)
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for metadata")
	}
	metaFile.SetChecksums(a.GetChecksums())

	// Files are downloaded as-is from docker images
	// we generate a new archive containing the required compressed file.
	// TODO: Bundle all the extra files in 1 docker image only, instead of an image for each file
	metaArchive, err := compiler.CreateArtifactForFile(a.GetPath())
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for tree")
	}
	imageMetaTree := fmt.Sprintf("%s:%s", imagePrefix, a.GetFileName())
	if opts, err := metaArchive.GenerateFinalImage(imageMetaTree, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating metadata tree"+opts.ImageName)
	}
	if r.PushImages {
		if err := pushImage(r.GetBackend(), imageMetaTree, true); err != nil {
			return errors.Wrapf(err, "Failed while pushing image: '%s'", imageMetaTree)
		}
	}
	data, err := yaml.Marshal(serialized)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(repospec, data, os.ModePerm)
	if err != nil {
		return err
	}

	tempRepoFile := filepath.Join(metaDir, REPOSITORY_SPECFILE+".tar")
	if err := helpers.Tar(repospec, tempRepoFile); err != nil {
		return errors.Wrap(err, "Error met while archiving repository file")
	}

	a = compiler.NewPackageArtifact(tempRepoFile)
	imageRepo := fmt.Sprintf("%s:%s", imagePrefix, REPOSITORY_SPECFILE)
	if opts, err := a.GenerateFinalImage(imageRepo, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating repository image"+opts.ImageName)
	}
	if r.PushImages {
		if err := pushImage(r.GetBackend(), imageRepo, true); err != nil {
			return errors.Wrapf(err, "Failed while pushing image: '%s'", imageRepo)
		}
	}

	bus.Manager.Publish(bus.EventRepositoryPostBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: imagePrefix,
	})
	return nil
}
