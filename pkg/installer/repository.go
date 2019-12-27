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
	"regexp"
	"sort"
	"strings"

	"github.com/mudler/luet/pkg/installer/client"

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

type LuetRepositorySerialized struct {
	Name     string                      `json:"name"`
	Uri      string                      `json:"uri"`
	Priority int                         `json:"priority"`
	Index    []*compiler.PackageArtifact `json:"index"`
	Type     string                      `json:"type"`
}

func GenerateRepository(name, uri, t string, priority int, src, treeDir string, db pkg.PackageDatabase) (Repository, error) {

	art, err := buildPackageIndex(src)
	if err != nil {
		return nil, err
	}
	tr := tree.NewInstallerRecipe(db)
	err = tr.Load(treeDir)
	if err != nil {
		return nil, err
	}

	return NewLuetRepository(name, uri, t, priority, art, tr), nil
}

func NewLuetRepository(name, uri, t string, priority int, art []compiler.Artifact, builder tree.Builder) Repository {
	return &LuetRepository{Index: art, Type: t, Tree: builder, Name: name, Uri: uri, Priority: priority}
}

func NewLuetRepositoryFromYaml(data []byte, db pkg.PackageDatabase) (Repository, error) {
	var p *LuetRepositorySerialized
	r := &LuetRepository{}
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}
	r.Name = p.Name
	r.Uri = p.Uri
	r.Priority = p.Priority
	r.Type = p.Type
	i := compiler.ArtifactIndex{}
	for _, ii := range p.Index {
		i = append(i, ii)
	}
	r.Index = i
	r.Tree = tree.NewInstallerRecipe(db)

	return r, err
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

func (r *LuetRepository) SetTree(b tree.Builder) {
	r.Tree = b
}

func (r *LuetRepository) GetType() string {
	return r.Type
}
func (r *LuetRepository) SetType(p string) {
	r.Type = p
}

func (r *LuetRepository) SetUri(p string) {
	r.Uri = p
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
	r.Index = r.Index.CleanPath()

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

func (r *LuetRepository) Client() Client {
	switch r.GetType() {
	case "disk":
		return client.NewLocalClient(client.RepoData{Uri: r.GetUri()})
	case "http":
		return client.NewHttpClient(client.RepoData{Uri: r.GetUri()})
	}

	return nil
}
func (r *LuetRepository) Sync() (Repository, error) {
	c := r.Client()
	if c == nil {
		return nil, errors.New("No client could be generated from repository.")
	}
	file, err := c.DownloadFile("repository.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "While downloading repository.yaml from "+r.GetUri())
	}
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+file)
	}
	defer os.Remove(file)

	// TODO: make it swappable
	repo, err := NewLuetRepositoryFromYaml(dat, pkg.NewInMemoryDatabase(false))
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

	reciper := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
	err = reciper.Load(treefs)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}
	repo.SetTree(reciper)
	repo.SetTreePath(treefs)
	repo.SetUri(r.GetUri())

	return repo, nil
}

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
		for _, p := range r[i].GetTree().GetDatabase().World() {
			cache[p.GetFingerPrint()] = p
		}
	}

	for _, v := range cache {
		world = append(world, v)
	}

	return world
}

func (r Repositories) SyncDatabase(d pkg.PackageDatabase) {
	cache := map[string]bool{}

	// Get Uniques. Walk in reverse so the definitions of most prio-repo overwrites lower ones
	// In this way, when we will walk again later the deps sorting them by most higher prio we have better chance of success.
	for i := len(r) - 1; i >= 0; i-- {
		for _, p := range r[i].GetTree().GetDatabase().World() {
			if _, ok := cache[p.GetFingerPrint()]; !ok {
				cache[p.GetFingerPrint()] = true
				d.CreatePackage(p)
			}
		}
	}

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
			c, err := r.GetTree().GetDatabase().FindPackage(pack)
			if err == nil {
				matches = append(matches, PackageMatch{Package: c, Repo: r})
				continue PACKAGE
			}
		}
	}

	return matches

}

func (re Repositories) Search(s string) []PackageMatch {
	sort.Sort(re)
	var term = regexp.MustCompile(s)
	var matches []PackageMatch

	for _, r := range re {
		for _, pack := range r.GetTree().GetDatabase().World() {
			if term.MatchString(pack.GetName()) {
				matches = append(matches, PackageMatch{Package: pack, Repo: r})
			}
		}
	}

	return matches
}
