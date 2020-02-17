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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	"github.com/mudler/luet/pkg/installer/client"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/ghodss/yaml"
	. "github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
)

const (
	REPOSITORY_SPECFILE = "repository.yaml"
	TREE_TARBALL        = "tree.tar"
)

type LuetSystemRepository struct {
	*config.LuetRepository

	Index               compiler.ArtifactIndex             `json:"index"`
	Tree                tree.Builder                       `json:"-"`
	TreePath            string                             `json:"treepath"`
	TreeCompressionType compiler.CompressionImplementation `json:"treecompressiontype"`
	TreeChecksums       compiler.Checksums                 `json:"treechecksums"`
}

type LuetSystemRepositorySerialized struct {
	Name                string                             `json:"name"`
	Description         string                             `json:"description,omitempty"`
	Urls                []string                           `json:"urls"`
	Priority            int                                `json:"priority"`
	Index               []*compiler.PackageArtifact        `json:"index"`
	Type                string                             `json:"type"`
	Revision            int                                `json:"revision,omitempty"`
	LastUpdate          string                             `json:"last_update,omitempty"`
	TreePath            string                             `json:"treepath"`
	TreeCompressionType compiler.CompressionImplementation `json:"treecompressiontype"`
	TreeChecksums       compiler.Checksums                 `json:"treechecksums"`
}

func GenerateRepository(name, descr, t string, urls []string, priority int, src, treeDir string, db pkg.PackageDatabase) (Repository, error) {

	art, err := buildPackageIndex(src)
	if err != nil {
		return nil, err
	}
	tr := tree.NewInstallerRecipe(db)
	err = tr.Load(treeDir)
	if err != nil {
		return nil, err
	}

	return NewLuetSystemRepository(
		config.NewLuetRepository(name, t, descr, urls, priority, true, false),
		art, tr), nil
}

func NewSystemRepository(repo config.LuetRepository) Repository {
	return &LuetSystemRepository{
		LuetRepository: &repo,
	}
}

func NewLuetSystemRepository(repo *config.LuetRepository, art []compiler.Artifact, builder tree.Builder) Repository {
	return &LuetSystemRepository{
		LuetRepository: repo,
		Index:          art,
		Tree:           builder,
	}
}

func NewLuetSystemRepositoryFromYaml(data []byte, db pkg.PackageDatabase) (Repository, error) {
	var p *LuetSystemRepositorySerialized
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}
	r := &LuetSystemRepository{
		LuetRepository: config.NewLuetRepository(
			p.Name,
			p.Type,
			p.Description,
			p.Urls,
			p.Priority,
			true,
			false,
		),
		TreeCompressionType: p.TreeCompressionType,
		TreeChecksums:       p.TreeChecksums,
		TreePath:            p.TreePath,
	}
	if p.Revision > 0 {
		r.Revision = p.Revision
	}
	if p.LastUpdate != "" {
		r.LastUpdate = p.LastUpdate
	}
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

func (r *LuetSystemRepository) GetName() string {
	return r.LuetRepository.Name
}
func (r *LuetSystemRepository) GetDescription() string {
	return r.LuetRepository.Description
}

func (r *LuetSystemRepository) GetAuthentication() map[string]string {
	return r.LuetRepository.Authentication
}

func (r *LuetSystemRepository) GetTreeCompressionType() compiler.CompressionImplementation {
	return r.TreeCompressionType
}

func (r *LuetSystemRepository) GetTreeChecksums() compiler.Checksums {
	return r.TreeChecksums
}

func (r *LuetSystemRepository) SetTreeCompressionType(c compiler.CompressionImplementation) {
	r.TreeCompressionType = c
}

func (r *LuetSystemRepository) SetTreeChecksums(c compiler.Checksums) {
	r.TreeChecksums = c
}

func (r *LuetSystemRepository) GetType() string {
	return r.LuetRepository.Type
}
func (r *LuetSystemRepository) SetType(p string) {
	r.LuetRepository.Type = p
}
func (r *LuetSystemRepository) AddUrl(p string) {
	r.LuetRepository.Urls = append(r.LuetRepository.Urls, p)
}
func (r *LuetSystemRepository) GetUrls() []string {
	return r.LuetRepository.Urls
}
func (r *LuetSystemRepository) SetUrls(urls []string) {
	r.LuetRepository.Urls = urls
}
func (r *LuetSystemRepository) GetPriority() int {
	return r.LuetRepository.Priority
}
func (r *LuetSystemRepository) GetTreePath() string {
	return r.TreePath
}
func (r *LuetSystemRepository) SetTreePath(p string) {
	r.TreePath = p
}
func (r *LuetSystemRepository) SetTree(b tree.Builder) {
	r.Tree = b
}
func (r *LuetSystemRepository) GetIndex() compiler.ArtifactIndex {
	return r.Index
}
func (r *LuetSystemRepository) GetTree() tree.Builder {
	return r.Tree
}
func (r *LuetSystemRepository) GetRevision() int {
	return r.LuetRepository.Revision
}
func (r *LuetSystemRepository) GetLastUpdate() string {
	return r.LuetRepository.LastUpdate
}
func (r *LuetSystemRepository) SetLastUpdate(u string) {
	r.LuetRepository.LastUpdate = u
}
func (r *LuetSystemRepository) IncrementRevision() {
	r.LuetRepository.Revision++
}

func (r *LuetSystemRepository) SetAuthentication(auth map[string]string) {
	r.LuetRepository.Authentication = auth
}

func (r *LuetSystemRepository) ReadSpecFile(file string, removeFile bool) (Repository, error) {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+file)
	}
	if removeFile {
		defer os.Remove(file)
	}

	var repo Repository
	repo, err = NewLuetSystemRepositoryFromYaml(dat, pkg.NewInMemoryDatabase(false))
	if err != nil {
		return nil, errors.Wrap(err, "Error reading repository from file "+file)
	}

	return repo, err
}

func (r *LuetSystemRepository) Write(dst string, resetRevision bool) error {
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return err
	}
	r.Index = r.Index.CleanPath()
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

	archive, err := ioutil.TempDir(os.TempDir(), "archive")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up
	err = r.GetTree().Save(archive)
	if err != nil {
		return errors.Wrap(err, "Error met while saving the tree")
	}
	tpath := r.GetTreePath()
	if tpath == "" {
		tpath = TREE_TARBALL
	}

	a := compiler.NewPackageArtifact(filepath.Join(dst, tpath))
	a.SetCompressionType(r.TreeCompressionType)
	err = a.Compress(archive, 1)
	if err != nil {
		return errors.Wrap(err, "Error met while creating package archive")
	}

	r.TreePath = path.Base(a.GetPath())
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for tree")
	}
	r.TreeChecksums = a.GetChecksums()

	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(repospec, data, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func (r *LuetSystemRepository) Client() Client {
	switch r.GetType() {
	case "disk":
		return client.NewLocalClient(client.RepoData{Urls: r.GetUrls()})
	case "http":
		return client.NewHttpClient(
			client.RepoData{
				Urls:           r.GetUrls(),
				Authentication: r.GetAuthentication(),
			})
	}

	return nil
}
func (r *LuetSystemRepository) Sync(force bool) (Repository, error) {
	var repoUpdated bool = false
	var treefs string

	Debug("Sync of the repository", r.Name, "in progress...")
	c := r.Client()
	if c == nil {
		return nil, errors.New("No client could be generated from repository.")
	}

	// Retrieve remote repository.yaml for retrieve revision and date
	file, err := c.DownloadFile(REPOSITORY_SPECFILE)
	if err != nil {
		return nil, errors.Wrap(err, "While downloading "+REPOSITORY_SPECFILE)
	}

	repobasedir := config.LuetCfg.GetSystem().GetRepoDatabaseDirPath(r.GetName())
	repo, err := r.ReadSpecFile(file, false)
	if err != nil {
		return nil, err
	}
	// Remove temporary file that contains repository.html.
	// Example: /tmp/HttpClient236052003
	defer os.RemoveAll(file)

	if r.Cached {
		if !force {
			localRepo, _ := r.ReadSpecFile(filepath.Join(repobasedir, REPOSITORY_SPECFILE), false)
			if localRepo != nil {
				if localRepo.GetRevision() == repo.GetRevision() &&
					localRepo.GetLastUpdate() == repo.GetLastUpdate() {
					repoUpdated = true
				}
			}
		}
		if r.GetTreePath() == "" {
			treefs = filepath.Join(repobasedir, "treefs")
		} else {
			treefs = r.GetTreePath()
		}

	} else {
		treefs, err = ioutil.TempDir(os.TempDir(), "treefs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
	}

	if !repoUpdated {
		tpath := repo.GetTreePath()
		if tpath == "" {
			tpath = TREE_TARBALL
		}
		a := compiler.NewPackageArtifact(tpath)

		artifact, err := c.DownloadArtifact(a)
		if err != nil {
			return nil, errors.Wrap(err, "While downloading "+tpath)
		}
		defer os.Remove(artifact.GetPath())

		artifact.SetChecksums(repo.GetTreeChecksums())
		artifact.SetCompressionType(repo.GetTreeCompressionType())

		err = artifact.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "Tree integrity check failure")
		}

		Debug("Tree tarball for the repository " + r.GetName() + " downloaded correctly.")

		if r.Cached {
			// Copy updated repository.yaml file to repo dir now that the tree is synced.
			err = helpers.CopyFile(file, filepath.Join(repobasedir, REPOSITORY_SPECFILE))
			if err != nil {
				return nil, errors.Wrap(err, "Error on update "+REPOSITORY_SPECFILE)
			}
			// Remove previous tree
			os.RemoveAll(treefs)
		}
		Debug("Decompress tree of the repository " + r.Name + "...")

		err = artifact.Unpack(treefs, true)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking tree")
		}

		tsec, _ := strconv.ParseInt(repo.GetLastUpdate(), 10, 64)

		InfoC(
			Bold(Red(":house: Repository "+r.GetName()+" revision: ")).String() +
				Bold(Green(repo.GetRevision())).String() + " - " +
				Bold(Green(time.Unix(tsec, 0).String())).String(),
		)

	} else {
		Info("Repository", r.GetName(), "is already up to date.")
	}

	reciper := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
	err = reciper.Load(treefs)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}

	repo.SetTree(reciper)
	repo.SetTreePath(treefs)
	repo.SetUrls(r.GetUrls())
	repo.SetAuthentication(r.GetAuthentication())

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

func (re Repositories) ResolveSelectors(p []pkg.Package) []pkg.Package {
	// If a selector is given, get the best from each repo
	sort.Sort(re) // respect prio
	var matches []pkg.Package
PACKAGE:
	for _, pack := range p {
	REPOSITORY:
		for _, r := range re {
			if pack.IsSelector() {
				c, err := r.GetTree().GetDatabase().FindPackageCandidate(pack)
				// If FindPackageCandidate returns the same package, it means it couldn't find one.
				// Skip this repository and keep looking.
				if c.String() == pack.String() {
					continue REPOSITORY
				}
				if err == nil {
					matches = append(matches, c)
					continue PACKAGE
				}
			} else {
				// If it's not a selector, just append it
				matches = append(matches, pack)
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
