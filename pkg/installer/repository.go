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

package installer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/mudler/luet/pkg/logger"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"
	"github.com/pkg/errors"
)

type LuetRepository struct {
	Name     string                 `json:"name"`
	Uri      string                 `json:"uri"`
	Priority int                    `json:"priority"`
	Index    compiler.ArtifactIndex `json:"index"`
	Tree     tree.Builder           `json:"-"`
	TreePath string                 `json:"-"`
	Type     string                 `json:"type"`
}

func GenerateRepository(name, uri string, priority int, src, tree string, db pkg.PackageDatabase) (Repository, error) {

	art, err := buildPackageIndex(src)
	if err != nil {
		return nil, err
	}

	return NewLuetRepository(name, uri, priority, art, db), nil
}

func NewLuetRepository(name, uri string, priority int, art []compiler.Artifact, db pkg.PackageDatabase) Repository {
	return &LuetRepository{Index: art, Tree: tree.NewInstallerRecipe(db)}
}

func NewLuetRepositoryFromYaml(data []byte) (Repository, error) {
	var p LuetRepository
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return &p, err
	}
	return &p, err
}

func buildPackageIndex(path string) ([]compiler.Artifact, error) {

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
		art = append(art, artifact)

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return nil, err

	}
	return art, nil
}

func (r *LuetRepository) GetName() string {
	return r.Name
}
func (r *LuetRepository) GetTreePath() string {
	return r.TreePath
}
func (r *LuetRepository) SetTreePath(p string) {
	r.TreePath = p
}

func (r *LuetRepository) GetType() string {
	return r.Type
}
func (r *LuetRepository) SetType(p string) {
	r.Type = p
}

func (r *LuetRepository) GetUri() string {
	return r.Uri
}
func (r *LuetRepository) GetPriority() int {
	return r.Priority
}
func (r *LuetRepository) GetIndex() compiler.ArtifactIndex {
	return r.Index
}
func (r *LuetRepository) GetTree() tree.Builder {
	return r.Tree
}

func (r *LuetRepository) Write(dst string) error {

	os.MkdirAll(dst, os.ModePerm)
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(dst, "repository.yaml"), data, os.ModePerm)
	if err != nil {
		return err
	}
	archive, err := ioutil.TempDir(os.TempDir(), "archive")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up
	err = r.GetTree().Save(archive)
	if err != nil {
		return errors.Wrap(err, "Error met while saving the tree")
	}
	err = helpers.Tar(archive, filepath.Join(dst, "tree.tar"))
	if err != nil {
		return errors.Wrap(err, "Error met while creating package archive")
	}
	return nil
}

func (r *LuetRepository) Sync(c Client) (Repository, error) {
	c.SetRepository(r)

	file, err := c.DownloadFile("repository.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "While downloading repository.yaml from "+r.GetUri())
	}
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+file)
	}
	defer os.Remove(file)

	repo, err := NewLuetRepositoryFromYaml(dat)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading repository from file "+file)

	}

	archivetree, err := c.DownloadFile("tree.tar")
	if err != nil {
		return nil, errors.Wrap(err, "While downloading repository.yaml from "+r.GetUri())
	}
	defer os.RemoveAll(archivetree) // clean up

	treefs, err := ioutil.TempDir(os.TempDir(), "treefs")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	//defer os.RemoveAll(treefs) // clean up

	// TODO: Following as option if archive as output?
	// archive, err := ioutil.TempDir(os.TempDir(), "archive")
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	// }
	// defer os.RemoveAll(archive) // clean up

	err = helpers.Untar(archivetree, treefs, false)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}

	reciper := tree.NewInstallerRecipe(r.GetTree().Tree().GetPackageSet())
	err = reciper.Load(treefs)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}
	repo.SetTreePath(treefs)

	return repo, nil
}

// TODO:

func (r Repositories) Len() int      { return len(r) }
func (r Repositories) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r Repositories) Less(i, j int) bool {
	return r[i].GetPriority() < r[j].GetPriority()
}

func (r Repositories) World() []pkg.Package {
	cache := map[string]pkg.Package{}
	world := []pkg.Package{}

	// Get Uniques. Walk in reverse so the definitions of most prio-repo overwrites lower ones
	// In this way, when we will walk again later the deps sorting them by most higher prio we have better chance of success.
	for i := len(r) - 1; i >= 0; i-- {
		w, err := r[i].GetTree().Tree().World()
		if err != nil {
			Warning("Failed computing world for " + r[i].GetName())
			continue
		}
		for _, p := range w {
			cache[p.GetFingerPrint()] = p
		}
	}

	for _, v := range cache {
		world = append(world, v)
	}

	return world
}

type PackageMatch struct {
	Repo    Repository
	Package pkg.Package
}

func (re Repositories) PackageMatches(p []pkg.Package) []PackageMatch {
	// TODO: Better heuristic. here we pick the first repo that contains the atom, sorted by priority but
	// we should do a permutations and get the best match, and in case there are more solutions the user should be able to pick
	sort.Sort(re)

	var matches []PackageMatch
PACKAGE:
	for _, pack := range p {
		for _, r := range re {
			c, err := r.GetTree().Tree().GetPackageSet().FindPackage(pack)
			if err == nil {
				matches = append(matches, PackageMatch{Package: c, Repo: r})
				continue PACKAGE
			}
		}
	}

	return matches

}
