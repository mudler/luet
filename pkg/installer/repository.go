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

	artifact "github.com/mudler/luet/pkg/compiler/types/artifact"
	compression "github.com/mudler/luet/pkg/compiler/types/compression"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"go.uber.org/multierr"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/installer/client"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

const (
	REPOSITORY_METAFILE  = "repository.meta.yaml"
	REPOSITORY_SPECFILE  = "repository.yaml"
	TREE_TARBALL         = "tree.tar"
	COMPILERTREE_TARBALL = "compilertree.tar"

	REPOFILE_TREE_KEY          = "tree"
	REPOFILE_COMPILER_TREE_KEY = "compilertree"
	REPOFILE_META_KEY          = "meta"

	DiskRepositoryType   = "disk"
	HttpRepositoryType   = "http"
	DockerRepositoryType = "docker"
)

type LuetRepositoryFile struct {
	FileName        string                     `json:"filename"`
	CompressionType compression.Implementation `json:"compressiontype,omitempty"`
	Checksums       artifact.Checksums         `json:"checksums,omitempty"`
}

type LuetSystemRepository struct {
	*config.LuetRepository

	Index           compiler.ArtifactIndex        `json:"index"`
	BuildTree, Tree tree.Builder                  `json:"-"`
	RepositoryFiles map[string]LuetRepositoryFile `json:"repo_files"`
	Backend         compiler.CompilerBackend      `json:"-"`
	PushImages      bool                          `json:"-"`
	ForcePush       bool                          `json:"-"`

	imagePrefix string
}

type LuetSystemRepositoryMetadata struct {
	Index []*artifact.PackageArtifact `json:"index,omitempty"`
}

type LuetSearchModeType int

const (
	SLabel      = iota
	SRegexPkg   = iota
	SRegexLabel = iota
	FileSearch  = iota
)

type LuetSearchOpts struct {
	Mode LuetSearchModeType
}

func NewLuetSystemRepositoryMetadata(file string, removeFile bool) (*LuetSystemRepositoryMetadata, error) {
	ans := &LuetSystemRepositoryMetadata{}
	err := ans.ReadFile(file, removeFile)
	if err != nil {
		return nil, err
	}
	return ans, nil
}

// SystemRepositories returns the repositories from the local configuration file
func SystemRepositories(c *config.LuetConfig) Repositories {
	repos := Repositories{}
	for _, repo := range c.SystemRepositories {
		if !repo.Enable {
			continue
		}
		r := NewSystemRepository(repo)
		repos = append(repos, r)
	}
	return repos
}

// LoadBuildTree loads to the tree the compilation specs from the system repositories
func LoadBuildTree(t tree.Builder, db pkg.PackageDatabase, c *config.LuetConfig) error {
	var reserr error
	repos := SystemRepositories(c)
	for _, r := range repos {
		repodir, err := config.LuetCfg.GetSystem().TempDir(r.Name)
		if err != nil {
			reserr = multierr.Append(reserr, err)
		}
		if err := r.SyncBuildMetadata(repodir); err != nil {
			reserr = multierr.Append(reserr, err)
		}

		generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
		if err := generalRecipe.Load(filepath.Join(repodir, "tree")); err != nil {
			reserr = multierr.Append(reserr, err)
		}
		if err := generalRecipe.GetDatabase().Clone(t.GetDatabase()); err != nil {
			reserr = multierr.Append(reserr, err)
		}

		r.SetTree(generalRecipe)
	}

	repos.SyncDatabase(db)

	return reserr
}

func (m *LuetSystemRepositoryMetadata) WriteFile(path string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, data, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func (m *LuetSystemRepositoryMetadata) ReadFile(file string, removeFile bool) error {
	if file == "" {
		return errors.New("Invalid path for repository metadata")
	}

	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if removeFile {
		defer os.Remove(file)
	}

	err = yaml.Unmarshal(dat, m)
	if err != nil {
		return err
	}

	return nil
}

func (m *LuetSystemRepositoryMetadata) ToArtifactIndex() (ans compiler.ArtifactIndex) {
	for _, a := range m.Index {
		ans = append(ans, a)
	}
	return
}

func NewDefaultTreeRepositoryFile() LuetRepositoryFile {
	return LuetRepositoryFile{
		FileName:        TREE_TARBALL,
		CompressionType: compression.GZip,
	}
}

func NewDefaultCompilerTreeRepositoryFile() LuetRepositoryFile {
	return LuetRepositoryFile{
		FileName:        COMPILERTREE_TARBALL,
		CompressionType: compression.GZip,
	}
}

func NewDefaultMetaRepositoryFile() LuetRepositoryFile {
	return LuetRepositoryFile{
		FileName:        REPOSITORY_METAFILE + ".tar",
		CompressionType: compression.None,
	}
}

// SetFileName sets the name of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this set the filename that the client will pull
func (f *LuetRepositoryFile) SetFileName(n string) {
	f.FileName = n
}

// GetFileName returns the name of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the filename that the client will pull
func (f *LuetRepositoryFile) GetFileName() string {
	return f.FileName
}

// SetCompressionType sets the compression type of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this sets the compression type that the client will use to uncompress the artifact
func (f *LuetRepositoryFile) SetCompressionType(c compression.Implementation) {
	f.CompressionType = c
}

// GetCompressionType gets the compression type of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the compression type that the client will use to uncompress the artifact
func (f *LuetRepositoryFile) GetCompressionType() compression.Implementation {
	return f.CompressionType
}

// SetChecksums sets the checksum of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this sets the checksums that the client will use to verify the artifact
func (f *LuetRepositoryFile) SetChecksums(c artifact.Checksums) {
	f.Checksums = c
}

// GetChecksums gets the checksum of the repository file.
// Each repository can ship arbitrary file that will be downloaded by the client
// in case of need, this gets the checksums that the client will use to verify the artifact
func (f *LuetRepositoryFile) GetChecksums() artifact.Checksums {
	return f.Checksums
}

// GenerateRepository generates a new repository from the given argument.
// If the repository is of the docker type, it will also push the package images.
// In case the repository is local, it will build the package Index
func GenerateRepository(p ...RepositoryOption) (*LuetSystemRepository, error) {
	c := RepositoryConfig{}
	c.Apply(p...)

	btr := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
	runtimeTree := pkg.NewInMemoryDatabase(false)

	tempTree := pkg.NewInMemoryDatabase(false)
	temptr := tree.NewInstallerRecipe(tempTree)

	for _, treeDir := range c.Tree {
		if err := temptr.Load(treeDir); err != nil {
			return nil, err
		}
		if err := btr.Load(treeDir); err != nil {
			return nil, err
		}
	}

	// 2: if fromRepo, build a new tree like the compiler is doing and use it to source the above specs,
	// instead of local tree

	repodb := pkg.NewInMemoryDatabase(false)
	generalRecipe := tree.NewCompilerRecipe(repodb)

	if c.FromRepository {
		if err := LoadBuildTree(generalRecipe, repodb, c.config); err != nil {
			Warning("errors while loading trees from repositories", err.Error())
		}

		if err := repodb.Clone(tempTree); err != nil {
			Warning("errors while cloning trees from repositories", err.Error())
		}

	}

	// Pick only atoms in db which have a real metadata for runtime db (tr)
	for _, p := range tempTree.World() {
		if _, err := os.Stat(filepath.Join(c.Src, p.GetMetadataFilePath())); err == nil {
			runtimeTree.CreatePackage(p)
		}
	}

	// Load packages from metadata files if not present already.
	var ff = func(currentpath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Only those which are metadata
		if !strings.HasSuffix(info.Name(), pkg.PackageMetaSuffix) {
			return nil
		}

		dat, err := ioutil.ReadFile(currentpath)
		if err != nil {
			return nil
		}

		art, err := artifact.NewPackageArtifactFromYaml(dat)
		if err != nil {
			return nil
		}
		if _, err := runtimeTree.FindPackage(art.CompileSpec.Package); err != nil && art.CompileSpec.Package.Name != "" {
			Debug("Adding", art.CompileSpec.Package.HumanReadableString(), "from metadata file", currentpath)
			if art.Runtime != nil && art.Runtime.Name != "" {
				runtimeTree.CreatePackage(art.Runtime)
			} else {
				// We don't have runtime at this point. So we import the package as is
				r := []*pkg.DefaultPackage{}
				p := art.CompileSpec.Package.Clone()
				p.Requires(r)
				p.SetProvides(r)
				p.Conflicts(r)
				runtimeTree.CreatePackage(p)
			}
		}

		return nil
	}

	if c.FromMetadata {
		// Best effort
		filepath.Walk(c.Src, ff)
	}

	repo := &LuetSystemRepository{
		LuetRepository:  config.NewLuetRepository(c.Name, c.Type, c.Description, c.Urls, c.Priority, true, false),
		Tree:            tree.NewInstallerRecipe(runtimeTree),
		BuildTree:       btr,
		RepositoryFiles: map[string]LuetRepositoryFile{},
		PushImages:      c.PushImages,
		ForcePush:       c.Force,
		Backend:         c.CompilerBackend,
		imagePrefix:     c.ImagePrefix,
	}

	if err := repo.initialize(c.Src); err != nil {
		return nil, errors.Wrap(err, "while building repository artifact index")
	}

	return repo, nil
}

func NewSystemRepository(repo config.LuetRepository) *LuetSystemRepository {
	return &LuetSystemRepository{
		LuetRepository:  &repo,
		RepositoryFiles: map[string]LuetRepositoryFile{},
	}
}

func NewLuetSystemRepositoryFromYaml(data []byte, db pkg.PackageDatabase) (*LuetSystemRepository, error) {
	var p *LuetSystemRepository
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}

	p.Tree = tree.NewInstallerRecipe(db)

	return p, err
}

func (r *LuetSystemRepository) SetPriority(n int) {
	r.LuetRepository.Priority = n
}

func (r *LuetSystemRepository) initialize(src string) error {
	generator, err := r.getGenerator()
	if err != nil {
		return errors.Wrap(err, "while constructing repository generator")
	}
	art, err := generator.Initialize(src, r.Tree.GetDatabase())
	if err != nil {
		return errors.Wrap(err, "while initializing repository generator")
	}
	// update the repository index
	r.Index = art
	return nil
}

// FileSearch search a pattern among the artifacts in a repository
func (r *LuetSystemRepository) FileSearch(pattern string) (pkg.Packages, error) {
	var matches pkg.Packages
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return matches, err
	}
ARTIFACT:
	for _, a := range r.GetIndex() {
		for _, f := range a.Files {
			if reg.MatchString(f) {
				matches = append(matches, a.CompileSpec.GetPackage())
				continue ARTIFACT
			}
		}
	}
	return matches, nil
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

func (r *LuetSystemRepository) GetType() string {
	return r.LuetRepository.Type
}

func (r *LuetSystemRepository) SetType(p string) {
	r.LuetRepository.Type = p
}

func (r *LuetSystemRepository) GetVerify() bool {
	return r.LuetRepository.Verify
}

func (r *LuetSystemRepository) SetVerify(p bool) {
	r.LuetRepository.Verify = p
}

func (r *LuetSystemRepository) GetBackend() compiler.CompilerBackend {
	return r.Backend
}
func (r *LuetSystemRepository) SetBackend(b compiler.CompilerBackend) {
	r.Backend = b
}

func (r *LuetSystemRepository) SetName(p string) {
	r.LuetRepository.Name = p
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
func (r *LuetSystemRepository) GetMetaPath() string {
	return r.MetaPath
}
func (r *LuetSystemRepository) SetMetaPath(p string) {
	r.MetaPath = p
}
func (r *LuetSystemRepository) SetTree(b tree.Builder) {
	r.Tree = b
}
func (r *LuetSystemRepository) GetIndex() compiler.ArtifactIndex {
	return r.Index
}
func (r *LuetSystemRepository) SetIndex(i compiler.ArtifactIndex) {
	r.Index = i
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

// BumpRevision bumps the internal repository revision by reading the current one from repospec
func (r *LuetSystemRepository) BumpRevision(repospec string, resetRevision bool) error {
	if resetRevision {
		r.Revision = 0
	} else {
		if _, err := os.Stat(repospec); !os.IsNotExist(err) {
			// Read existing file for retrieve revision
			spec, err := r.ReadSpecFile(repospec)
			if err != nil {
				return err
			}
			r.Revision = spec.GetRevision()
		}
	}
	r.Revision++
	return nil
}

// AddMetadata adds the repository serialized content into the metadata key of the repository
// It writes the serialized content to repospec, and writes the repository.meta.yaml file into dst
func (r *LuetSystemRepository) AddMetadata(repospec, dst string) (*artifact.PackageArtifact, error) {
	// Create Metadata struct and serialized repository
	meta, serialized := r.Serialize()

	// Create metadata file and repository file
	metaTmpDir, err := config.LuetCfg.GetSystem().TempDir("metadata")
	defer os.RemoveAll(metaTmpDir) // clean up
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for metadata")
	}

	repoMetaSpec := filepath.Join(metaTmpDir, REPOSITORY_METAFILE)

	// Create repository.meta.yaml file
	err = meta.WriteFile(repoMetaSpec)
	if err != nil {
		return nil, err
	}
	a, err := r.AddRepositoryFile(metaTmpDir, REPOFILE_META_KEY, dst, NewDefaultMetaRepositoryFile())
	if err != nil {
		return a, errors.Wrap(err, "Error met while adding archive to repository")
	}

	data, err := yaml.Marshal(serialized)
	if err != nil {
		return a, err
	}
	err = ioutil.WriteFile(repospec, data, os.ModePerm)
	if err != nil {
		return a, err
	}
	return a, nil
}

// AddTree adds a tree.Builder with the given key to the repository.
// It will generate an artifact which will be then embedded in the repository manifest
// It returns the generated artifacts and an error
func (r *LuetSystemRepository) AddTree(t tree.Builder, dst, key string, f LuetRepositoryFile) (*artifact.PackageArtifact, error) {
	// Create tree and repository file
	archive, err := config.LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up

	if err := t.Save(archive); err != nil {
		return nil, errors.Wrap(err, "Error met while saving the tree")
	}

	a, err := r.AddRepositoryFile(archive, key, dst, f)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while adding archive to repository")
	}
	return a, nil
}

// AddRepositoryFile adds a path to a key in the repository manifest.
// The path will be compressed, and a default File has to be passed in case there is no entry into
// the repository manifest
func (r *LuetSystemRepository) AddRepositoryFile(src, fileKey, repositoryRoot string, defaults LuetRepositoryFile) (*artifact.PackageArtifact, error) {
	treeFile, err := r.GetRepositoryFile(fileKey)
	if err != nil {
		treeFile = defaults
		//	r.SetRepositoryFile(fileKey, treeFile)
	}

	a := artifact.NewPackageArtifact(filepath.Join(repositoryRoot, treeFile.GetFileName()))
	a.CompressionType = treeFile.GetCompressionType()
	err = a.Compress(src, 1)
	if err != nil {
		return a, errors.Wrap(err, "Error met while creating package archive")
	}

	err = a.Hash()
	if err != nil {
		return a, errors.Wrap(err, "Failed generating checksums for tree")
	}
	// Update the tree name with the name created by compression selected.
	treeFile.SetChecksums(a.Checksums)
	treeFile.SetFileName(path.Base(a.Path))

	r.SetRepositoryFile(fileKey, treeFile)

	return a, nil
}

func (r *LuetSystemRepository) GetRepositoryFile(name string) (LuetRepositoryFile, error) {
	ans, ok := r.RepositoryFiles[name]
	if ok {
		return ans, nil
	}
	return ans, errors.New("Repository file " + name + " not found!")
}
func (r *LuetSystemRepository) SetRepositoryFile(name string, f LuetRepositoryFile) {
	r.RepositoryFiles[name] = f
}

func (r *LuetSystemRepository) ReadSpecFile(file string) (*LuetSystemRepository, error) {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+file)
	}
	var repo *LuetSystemRepository
	repo, err = NewLuetSystemRepositoryFromYaml(dat, pkg.NewInMemoryDatabase(false))
	if err != nil {
		return nil, errors.Wrap(err, "Error reading repository from file "+file)
	}

	// Check if mandatory key are present
	_, err = repo.GetRepositoryFile(REPOFILE_TREE_KEY)
	if err != nil {
		return nil, errors.New("Invalid repository without the " + REPOFILE_TREE_KEY + " key file.")
	}
	_, err = repo.GetRepositoryFile(REPOFILE_META_KEY)
	if err != nil {
		return nil, errors.New("Invalid repository without the " + REPOFILE_META_KEY + " key file.")
	}

	return repo, err
}

type RepositoryGenerator interface {
	Generate(*LuetSystemRepository, string, bool) error
	Initialize(string, pkg.PackageDatabase) ([]*artifact.PackageArtifact, error)
}

func (r *LuetSystemRepository) getGenerator() (RepositoryGenerator, error) {
	var rg RepositoryGenerator
	switch r.GetType() {
	case DiskRepositoryType, HttpRepositoryType:
		rg = &localRepositoryGenerator{}
	case DockerRepositoryType:
		rg = &dockerRepositoryGenerator{
			b:           r.Backend,
			imagePrefix: r.imagePrefix,
			imagePush:   r.PushImages,
			force:       r.ForcePush,
		}
	default:
		return nil, errors.New("invalid repository type")
	}
	return rg, nil
}

// Write writes the repository metadata to the supplied destination
func (r *LuetSystemRepository) Write(dst string, resetRevision, force bool) error {
	rg, err := r.getGenerator()
	if err != nil {
		return err
	}

	return rg.Generate(r, dst, resetRevision)
}

func (r *LuetSystemRepository) Client() Client {
	switch r.GetType() {
	case DiskRepositoryType:
		return client.NewLocalClient(client.RepoData{Urls: r.GetUrls()})
	case HttpRepositoryType:
		return client.NewHttpClient(
			client.RepoData{
				Urls:           r.GetUrls(),
				Authentication: r.GetAuthentication(),
			})

	case DockerRepositoryType:
		return client.NewDockerClient(
			client.RepoData{
				Urls:           r.GetUrls(),
				Authentication: r.GetAuthentication(),
				Verify:         r.Verify,
			})
	}
	return nil
}

func (r *LuetSystemRepository) SearchArtefact(p pkg.Package) (*artifact.PackageArtifact, error) {
	for _, a := range r.GetIndex() {
		if a.CompileSpec.GetPackage().Matches(p) {
			return a, nil
		}
	}

	return nil, errors.New("Not found")
}

func (r *LuetSystemRepository) getRepoFile(c Client, key string) (*artifact.PackageArtifact, error) {

	treeFile, err := r.GetRepositoryFile(key)
	if err != nil {
		return nil, errors.Wrapf(err, "key %s not present in the repository", key)
	}

	// Get Tree
	downloadedTreeFile, err := c.DownloadFile(treeFile.GetFileName())
	if err != nil {
		return nil, errors.Wrap(err, "While downloading "+treeFile.GetFileName())
	}
	//defer os.Remove(downloadedTreeFile)

	treeFileArtifact := artifact.NewPackageArtifact(downloadedTreeFile)
	treeFileArtifact.Checksums = treeFile.GetChecksums()
	treeFileArtifact.CompressionType = treeFile.GetCompressionType()

	err = treeFileArtifact.Verify()
	if err != nil {
		return nil, errors.Wrap(err, "file integrity check failure")
	}

	return treeFileArtifact, nil

}

func (r *LuetSystemRepository) SyncBuildMetadata(path string) error {

	repo, err := r.Sync(false)
	if err != nil {
		return errors.Wrap(err, "while syncronizing repository")
	}

	c := repo.Client()
	if c == nil {
		return errors.New("no client could be generated from repository")
	}

	a, err := repo.getRepoFile(c, REPOFILE_COMPILER_TREE_KEY)
	if err != nil {
		return fmt.Errorf("failed while getting: %s", REPOFILE_COMPILER_TREE_KEY)
	}

	defer os.RemoveAll(a.Path)

	if err := a.Unpack(filepath.Join(path, "tree"), false); err != nil {
		return errors.Wrapf(err, "while unpacking: %s", REPOFILE_COMPILER_TREE_KEY)
	}

	for _, ai := range repo.GetTree().GetDatabase().World() {
		// Retrieve remote repository.yaml for retrieve revision and date
		file, err := c.DownloadFile(ai.GetMetadataFilePath())
		if err != nil {
			return errors.Wrapf(err, "while downloading metadata for %s", ai.HumanReadableString())
		}
		if err := fileHelper.Move(file, filepath.Join(path, ai.GetMetadataFilePath())); err != nil {
			return err
		}
	}

	return nil
}

func (r *LuetSystemRepository) Sync(force bool) (*LuetSystemRepository, error) {
	var repoUpdated bool = false
	var treefs, metafs string
	aurora := GetAurora()

	Debug("Sync of the repository", r.Name, "in progress...")
	c := r.Client()
	if c == nil {
		return nil, errors.New("no client could be generated from repository")
	}

	// Retrieve remote repository.yaml for retrieve revision and date
	file, err := c.DownloadFile(REPOSITORY_SPECFILE)
	if err != nil {
		return nil, errors.Wrap(err, "While downloading "+REPOSITORY_SPECFILE)
	}

	repobasedir := config.LuetCfg.GetSystem().GetRepoDatabaseDirPath(r.GetName())
	downloadedRepoMeta, err := r.ReadSpecFile(file)
	if err != nil {
		return nil, err
	}
	// Remove temporary file that contains repository.yaml
	// Example: /tmp/HttpClient236052003
	defer os.RemoveAll(file)

	if r.Cached {
		if !force {
			localRepo, _ := r.ReadSpecFile(filepath.Join(repobasedir, REPOSITORY_SPECFILE))
			if localRepo != nil {
				if localRepo.GetRevision() == downloadedRepoMeta.GetRevision() &&
					localRepo.GetLastUpdate() == downloadedRepoMeta.GetLastUpdate() {
					repoUpdated = true
				}
			}
		}
		if r.GetTreePath() == "" {
			treefs = filepath.Join(repobasedir, "treefs")
		} else {
			treefs = r.GetTreePath()
		}
		if r.GetMetaPath() == "" {
			metafs = filepath.Join(repobasedir, "metafs")
		} else {
			metafs = r.GetMetaPath()
		}

	} else {
		treefs, err = config.LuetCfg.GetSystem().TempDir("treefs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
		metafs, err = config.LuetCfg.GetSystem().TempDir("metafs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met whilte creating tempdir for metafs")
		}
	}

	// treeFile and metaFile must be present, they aren't optional
	if !repoUpdated {

		treeFileArtifact, err := downloadedRepoMeta.getRepoFile(c, REPOFILE_TREE_KEY)
		if err != nil {
			return nil, errors.Wrapf(err, "while fetching '%s'", REPOFILE_TREE_KEY)
		}
		defer os.Remove(treeFileArtifact.Path)

		Debug("Tree tarball for the repository " + r.GetName() + " downloaded correctly.")

		metaFileArtifact, err := downloadedRepoMeta.getRepoFile(c, REPOFILE_META_KEY)
		if err != nil {
			return nil, errors.Wrapf(err, "while fetching '%s'", REPOFILE_META_KEY)
		}
		defer os.Remove(metaFileArtifact.Path)

		Debug("Metadata tarball for the repository " + r.GetName() + " downloaded correctly.")

		if r.Cached {
			// Copy updated repository.yaml file to repo dir now that the tree is synced.
			err = fileHelper.CopyFile(file, filepath.Join(repobasedir, REPOSITORY_SPECFILE))
			if err != nil {
				return nil, errors.Wrap(err, "Error on update "+REPOSITORY_SPECFILE)
			}
			// Remove previous tree
			os.RemoveAll(treefs)
			// Remove previous meta dir
			os.RemoveAll(metafs)
		}
		Debug("Decompress tree of the repository " + r.Name + "...")

		err = treeFileArtifact.Unpack(treefs, true)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking tree")
		}

		// FIXME: It seems that tar with only one file doesn't create destination
		//       directory. I create directory directly for now.
		os.MkdirAll(metafs, os.ModePerm)
		err = metaFileArtifact.Unpack(metafs, true)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking metadata")
		}

		tsec, _ := strconv.ParseInt(downloadedRepoMeta.GetLastUpdate(), 10, 64)

		InfoC(
			aurora.Bold(
				aurora.Red(":house: Repository "+downloadedRepoMeta.GetName()+" revision: ")).String() +
				aurora.Bold(aurora.Green(downloadedRepoMeta.GetRevision())).String() + " - " +
				aurora.Bold(aurora.Green(time.Unix(tsec, 0).String())).String(),
		)

	} else {
		Info("Repository", downloadedRepoMeta.GetName(), "is already up to date.")
	}

	meta, err := NewLuetSystemRepositoryMetadata(
		filepath.Join(metafs, REPOSITORY_METAFILE), false,
	)
	if err != nil {
		return nil, errors.Wrap(err, "While processing "+REPOSITORY_METAFILE)
	}
	downloadedRepoMeta.SetIndex(meta.ToArtifactIndex())

	reciper := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
	err = reciper.Load(treefs)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}

	downloadedRepoMeta.SetTree(reciper)
	downloadedRepoMeta.SetTreePath(treefs)

	// Copy the local available data to the one which was synced
	// e.g. locally we can override the type (disk), or priority
	// while remotely it could be advertized differently
	r.fill(downloadedRepoMeta)

	InfoC(
		aurora.Yellow(":information_source:").String() +
			aurora.Magenta("Repository: ").String() +
			aurora.Green(aurora.Bold(downloadedRepoMeta.GetName()).String()).String() +
			aurora.Magenta(" Priority: ").String() +
			aurora.Bold(aurora.Green(downloadedRepoMeta.GetPriority())).String() +
			aurora.Magenta(" Type: ").String() +
			aurora.Bold(aurora.Green(downloadedRepoMeta.GetType())).String(),
	)
	return downloadedRepoMeta, nil
}

func (r *LuetSystemRepository) fill(r2 *LuetSystemRepository) {
	r2.SetUrls(r.GetUrls())
	r2.SetAuthentication(r.GetAuthentication())
	r2.SetType(r.GetType())
	r2.SetPriority(r.GetPriority())
	r2.SetName(r.GetName())
	r2.SetVerify(r.GetVerify())
}

func (r *LuetSystemRepository) Serialize() (*LuetSystemRepositoryMetadata, LuetSystemRepository) {

	serialized := *r
	serialized.Authentication = nil

	serialized.Index = compiler.ArtifactIndex{}

	meta := &LuetSystemRepositoryMetadata{
		Index: []*artifact.PackageArtifact{},
	}
	for _, a := range r.Index {
		cp := *a
		copy := &cp
		copy.Path = filepath.Base(copy.Path)
		meta.Index = append(meta.Index, copy)
	}

	return meta, serialized
}

func (r Repositories) Len() int      { return len(r) }
func (r Repositories) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r Repositories) Less(i, j int) bool {
	return r[i].GetPriority() < r[j].GetPriority()
}

func (r Repositories) World() pkg.Packages {
	cache := map[string]pkg.Package{}
	world := pkg.Packages{}

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
	Repo     *LuetSystemRepository
	Artifact *artifact.PackageArtifact
	Package  pkg.Package
}

func (re Repositories) PackageMatches(p pkg.Packages) []PackageMatch {
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

func (re Repositories) ResolveSelectors(p pkg.Packages) pkg.Packages {
	// If a selector is given, get the best from each repo
	sort.Sort(re) // respect prio
	var matches pkg.Packages
PACKAGE:
	for _, pack := range p {
	REPOSITORY:
		for _, r := range re {
			if pack.IsSelector() {
				c, err := r.GetTree().GetDatabase().FindPackageCandidate(pack)
				// If FindPackageCandidate returns the same package, it means it couldn't find one.
				// Skip this repository and keep looking.
				if err != nil { //c.String() == pack.String() {
					continue REPOSITORY
				}
				matches = append(matches, c)
				continue PACKAGE
			} else {
				// If it's not a selector, just append it
				matches = append(matches, pack)
			}
		}
	}

	return matches

}

func (re Repositories) SearchPackages(p string, t LuetSearchModeType) []PackageMatch {
	sort.Sort(re)
	var matches []PackageMatch
	var err error

	for _, r := range re {
		var repoMatches pkg.Packages

		switch t {
		case SRegexPkg:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageMatch(p)
		case SLabel:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageLabel(p)
		case SRegexLabel:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageLabelMatch(p)
		case FileSearch:
			repoMatches, err = r.FileSearch(p)
		}

		if err == nil && len(repoMatches) > 0 {
			for _, pack := range repoMatches {
				a, _ := r.SearchArtefact(pack)
				matches = append(matches, PackageMatch{Package: pack, Repo: r, Artifact: a})
			}
		}
	}

	return matches
}

func (re Repositories) SearchLabelMatch(s string) []PackageMatch {
	return re.SearchPackages(s, SRegexLabel)
}

func (re Repositories) SearchLabel(s string) []PackageMatch {
	return re.SearchPackages(s, SLabel)
}

func (re Repositories) Search(s string) []PackageMatch {
	return re.SearchPackages(s, SRegexPkg)
}
