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

	"github.com/mudler/luet/pkg/bus"
	compiler "github.com/mudler/luet/pkg/compiler"
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

// Generate creates a Local luet repository
func (*localRepositoryGenerator) Generate(r *LuetSystemRepository, dst string, resetRevision bool) error {
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return err
	}
	r.LastUpdate = strconv.FormatInt(time.Now().Unix(), 10)

	repospec := filepath.Join(dst, REPOSITORY_SPECFILE)
	// Increment the internal revision version by reading the one which is already available (if any)
	if err := r.BumpRevision(repospec, resetRevision); err != nil {
		return err
	}

	Info(fmt.Sprintf(
		"Repository %s: creating revision %d and last update %s...",
		r.Name, r.Revision, r.LastUpdate,
	))

	bus.Manager.Publish(bus.EventRepositoryPreBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: dst,
	})

	if _, err := r.AddTree(r.GetTree(), dst, REPOFILE_TREE_KEY, NewDefaultTreeRepositoryFile()); err != nil {
		return errors.Wrap(err, "error met while adding runtime tree to repository")
	}

	if _, err := r.AddTree(r.BuildTree, dst, REPOFILE_COMPILER_TREE_KEY, NewDefaultCompilerTreeRepositoryFile()); err != nil {
		return errors.Wrap(err, "error met while adding compiler tree to repository")
	}

	if _, err := r.AddMetadata(repospec, dst); err != nil {
		return errors.Wrap(err, "failed adding Metadata file to repository")
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
