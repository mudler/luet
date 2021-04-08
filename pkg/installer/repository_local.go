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
	"path"
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
	"github.com/pkg/errors"
)

type localRepositoryGenerator struct{}

func (l *localRepositoryGenerator) Initialize(path string, db pkg.PackageDatabase) ([]compiler.Artifact, error) {
	return buildPackageIndex(path, db)
}

func buildPackageIndex(path string, db pkg.PackageDatabase) ([]compiler.Artifact, error) {

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

		// We want to include packages that are ONLY referenced in the tree.
		// the ones which aren't should be deleted. (TODO: by another cli command?)
		if _, notfound := db.FindPackage(artifact.GetCompileSpec().GetPackage()); notfound != nil {
			Debug(fmt.Sprintf("Package %s not found in tree. Ignoring it.",
				artifact.GetCompileSpec().GetPackage().HumanReadableString()))
			return nil
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

func (*localRepositoryGenerator) Generate(r *LuetSystemRepository, dst string, resetRevision bool) error {
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return err
	}
	r.LastUpdate = strconv.FormatInt(time.Now().Unix(), 10)

	repospec := filepath.Join(dst, REPOSITORY_SPECFILE)
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
		Path: dst,
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

	a := compiler.NewPackageArtifact(filepath.Join(dst, treeFile.GetFileName()))
	a.SetCompressionType(treeFile.GetCompressionType())
	err = a.Compress(archive, 1)
	if err != nil {
		return errors.Wrap(err, "Error met while creating package archive")
	}

	// Update the tree name with the name created by compression selected.
	treeFile.SetFileName(path.Base(a.GetPath()))
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for tree")
	}
	treeFile.SetChecksums(a.GetChecksums())
	r.SetRepositoryFile(REPOFILE_TREE_KEY, treeFile)

	// Create Metadata struct and serialized repository
	meta, serialized := r.Serialize()

	// Create metadata file and repository file
	metaTmpDir, err := config.LuetCfg.GetSystem().TempDir("metadata")
	defer os.RemoveAll(metaTmpDir) // clean up
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for metadata")
	}

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

	a = compiler.NewPackageArtifact(filepath.Join(dst, metaFile.GetFileName()))
	a.SetCompressionType(metaFile.GetCompressionType())
	err = a.Compress(metaTmpDir, 1)
	if err != nil {
		return errors.Wrap(err, "Error met while archiving repository metadata")
	}

	metaFile.SetFileName(path.Base(a.GetPath()))
	r.SetRepositoryFile(REPOFILE_META_KEY, metaFile)
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for metadata")
	}
	metaFile.SetChecksums(a.GetChecksums())

	data, err := yaml.Marshal(serialized)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(repospec, data, os.ModePerm)
	if err != nil {
		return err
	}

	bus.Manager.Publish(bus.EventRepositoryPostBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: dst,
	})
	return nil
}
