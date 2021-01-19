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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/luet/pkg/bus"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	"github.com/mudler/luet/pkg/installer/client"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	tree "github.com/mudler/luet/pkg/tree"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

const (
	REPOSITORY_METAFILE = "repository.meta.yaml"
	REPOSITORY_SPECFILE = "repository.yaml"
	TREE_TARBALL        = "tree.tar"

	REPOFILE_TREE_KEY = "tree"
	REPOFILE_META_KEY = "meta"

	DiskRepositoryType   = "disk"
	HttpRepositoryType   = "http"
	DockerRepositoryType = "docker"
)

type LuetRepositoryFile struct {
	FileName        string                             `json:"filename"`
	CompressionType compiler.CompressionImplementation `json:"compressiontype,omitempty"`
	Checksums       compiler.Checksums                 `json:"checksums,omitempty"`
}

type LuetSystemRepository struct {
	*config.LuetRepository

	Index           compiler.ArtifactIndex        `json:"index"`
	Tree            tree.Builder                  `json:"-"`
	RepositoryFiles map[string]LuetRepositoryFile `json:"repo_files"`
	Backend         compiler.CompilerBackend      `json:"-"`
}

type LuetSystemRepositorySerialized struct {
	Name            string                        `json:"name"`
	Description     string                        `json:"description,omitempty"`
	Urls            []string                      `json:"urls"`
	Priority        int                           `json:"priority"`
	Type            string                        `json:"type"`
	Revision        int                           `json:"revision,omitempty"`
	LastUpdate      string                        `json:"last_update,omitempty"`
	TreePath        string                        `json:"treepath"`
	MetaPath        string                        `json:"metapath"`
	RepositoryFiles map[string]LuetRepositoryFile `json:"repo_files"`
}

type LuetSystemRepositoryMetadata struct {
	Index []*compiler.PackageArtifact `json:"index,omitempty"`
}

type LuetSearchModeType string

const (
	SLabel      LuetSearchModeType = "label"
	SRegexPkg   LuetSearchModeType = "regexPkg"
	SRegexLabel LuetSearchModeType = "regexLabel"
)

type LuetSearchOpts struct {
	Pattern string
	Mode    LuetSearchModeType
}

func NewLuetSystemRepositoryMetadata(file string, removeFile bool) (*LuetSystemRepositoryMetadata, error) {
	ans := &LuetSystemRepositoryMetadata{}
	err := ans.ReadFile(file, removeFile)
	if err != nil {
		return nil, err
	}
	return ans, nil
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
		CompressionType: compiler.GZip,
	}
}

func NewDefaultMetaRepositoryFile() LuetRepositoryFile {
	return LuetRepositoryFile{
		FileName:        REPOSITORY_METAFILE + ".tar",
		CompressionType: compiler.None,
	}
}

func (f *LuetRepositoryFile) SetFileName(n string) {
	f.FileName = n
}

func (f *LuetRepositoryFile) GetFileName() string {
	return f.FileName
}
func (f *LuetRepositoryFile) SetCompressionType(c compiler.CompressionImplementation) {
	f.CompressionType = c
}
func (f *LuetRepositoryFile) GetCompressionType() compiler.CompressionImplementation {
	return f.CompressionType
}
func (f *LuetRepositoryFile) SetChecksums(c compiler.Checksums) {
	f.Checksums = c
}
func (f *LuetRepositoryFile) GetChecksums() compiler.Checksums {
	return f.Checksums
}

func GenerateRepository(name, descr, t string, urls []string,
	priority int, src string, treesDir []string, db pkg.PackageDatabase,
	b compiler.CompilerBackend, imagePrefix string) (Repository, error) {

	tr := tree.NewInstallerRecipe(db)

	for _, treeDir := range treesDir {
		err := tr.Load(treeDir)
		if err != nil {
			return nil, err
		}
	}

	var art []compiler.Artifact
	var err error
	switch t {
	case DiskRepositoryType, HttpRepositoryType:
		art, err = buildPackageIndex(src, tr.GetDatabase())
		if err != nil {
			return nil, err
		}

	case DockerRepositoryType:
		art, err = generatePackageImages(b, imagePrefix, src, tr.GetDatabase())
		if err != nil {
			return nil, err
		}
	}

	repo := NewLuetSystemRepository(
		config.NewLuetRepository(name, t, descr, urls, priority, true, false),
		art, tr)
	repo.SetBackend(b)
	return repo, nil
}

func NewSystemRepository(repo config.LuetRepository) Repository {
	return &LuetSystemRepository{
		LuetRepository:  &repo,
		RepositoryFiles: map[string]LuetRepositoryFile{},
	}
}

func NewLuetSystemRepository(repo *config.LuetRepository, art []compiler.Artifact, builder tree.Builder) Repository {
	return &LuetSystemRepository{
		LuetRepository:  repo,
		Index:           art,
		Tree:            builder,
		RepositoryFiles: map[string]LuetRepositoryFile{},
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
		RepositoryFiles: p.RepositoryFiles,
	}
	if p.Revision > 0 {
		r.Revision = p.Revision
	}
	if p.LastUpdate != "" {
		r.LastUpdate = p.LastUpdate
	}
	r.Tree = tree.NewInstallerRecipe(db)

	return r, err
}

func generatePackageImages(b compiler.CompilerBackend, imagePrefix, path string, db pkg.PackageDatabase) ([]compiler.Artifact, error) {

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

		Info("Generating final image", imagePrefix+artifact.GetCompileSpec().GetPackage().GetPackageImageName(),
			"for package ", artifact.GetCompileSpec().GetPackage().HumanReadableString())
		if opts, err := artifact.GenerateFinalImage(imagePrefix+artifact.GetCompileSpec().GetPackage().GetPackageImageName(), b, true); err != nil {
			return errors.Wrap(err, "Failed generating metadata tree"+opts.ImageName)
		}
		// TODO: Push image (check if exists first, and avoid to re-push the same images, unless --force is passed)

		art = append(art, artifact)

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return nil, err

	}
	return art, nil
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

func (r *LuetSystemRepository) SetPriority(n int) {
	r.LuetRepository.Priority = n
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

func (r *LuetSystemRepository) genLocalRepo(dst string, resetRevision bool) error {
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

func (r *LuetSystemRepository) genDockerRepo(imagePrefix string, resetRevision, force bool) error {
	// - Iterate over meta, build final images, push them if necessary
	//   - while pushing, check if image already exists, and if exist push them only if --force is supplied
	// - Generate final images for metadata and push

	imageRepository := fmt.Sprintf("%s%s", imagePrefix, "repository")

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

	treeFile := NewDefaultTreeRepositoryFile()
	a := compiler.NewPackageArtifact(filepath.Join(repoTemp, treeFile.GetFileName()))
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

	imageTree := fmt.Sprintf("%s%s:%s", imagePrefix, "repository", TREE_TARBALL)
	Debug("Generating image", imageTree)
	if opts, err := a.GenerateFinalImage(imageTree, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating metadata tree "+opts.ImageName)
	}
	// TODO: Push imageTree

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

	metaFile.SetFileName(path.Base(a.GetPath()))
	r.SetRepositoryFile(REPOFILE_META_KEY, metaFile)
	err = a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for metadata")
	}
	metaFile.SetChecksums(a.GetChecksums())

	imageMetaTree := fmt.Sprintf("%s%s:%s", imagePrefix, "repository", REPOSITORY_METAFILE)
	if opts, err := a.GenerateFinalImage(imageMetaTree, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating metadata tree"+opts.ImageName)
	}

	// TODO: Push image meta tree
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
	imageRepo := fmt.Sprintf("%s%s:%s", imagePrefix, "repository", REPOSITORY_SPECFILE)
	if opts, err := a.GenerateFinalImage(imageRepo, r.GetBackend(), false); err != nil {
		return errors.Wrap(err, "Failed generating repository image"+opts.ImageName)
	}
	// TODO: Push image meta tree

	bus.Manager.Publish(bus.EventRepositoryPostBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: imagePrefix,
	})
	return nil
}

// Write writes the repository metadata to the supplied destination
func (r *LuetSystemRepository) Write(dst string, resetRevision, force bool) error {
	switch r.GetType() {
	case DiskRepositoryType, HttpRepositoryType:
		return r.genLocalRepo(dst, resetRevision)
	case DockerRepositoryType:
		return r.genDockerRepo(dst, resetRevision, force)
	}
	return errors.New("invalid repository type")
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
	}

	return nil
}

func (r *LuetSystemRepository) Sync(force bool) (Repository, error) {
	var repoUpdated bool = false
	var treefs, metafs string
	aurora := GetAurora()

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
	// Remove temporary file that contains repository.yaml
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

	// POST: treeFile and metaFile are present. I check this inside
	// ReadSpecFile and NewLuetSystemRepositoryFromYaml
	treeFile, _ := repo.GetRepositoryFile(REPOFILE_TREE_KEY)
	metaFile, _ := repo.GetRepositoryFile(REPOFILE_META_KEY)

	if !repoUpdated {

		// Get Tree
		downloadedTreeFile, err := c.DownloadFile(treeFile.GetFileName())
		if err != nil {
			return nil, errors.Wrap(err, "While downloading "+treeFile.GetFileName())
		}
		defer os.Remove(downloadedTreeFile)

		// Treat the file as artifact, in order to verify it
		treeFileArtifact := compiler.NewPackageArtifact(downloadedTreeFile)
		treeFileArtifact.SetChecksums(treeFile.GetChecksums())
		treeFileArtifact.SetCompressionType(treeFile.GetCompressionType())

		err = treeFileArtifact.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "Tree integrity check failure")
		}

		Debug("Tree tarball for the repository " + r.GetName() + " downloaded correctly.")

		// Get Repository Metadata
		downloadedMeta, err := c.DownloadFile(metaFile.GetFileName())
		if err != nil {
			return nil, errors.Wrap(err, "While downloading "+metaFile.GetFileName())
		}
		defer os.Remove(downloadedMeta)

		metaFileArtifact := compiler.NewPackageArtifact(downloadedMeta)
		metaFileArtifact.SetChecksums(metaFile.GetChecksums())
		metaFileArtifact.SetCompressionType(metaFile.GetCompressionType())

		err = metaFileArtifact.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "Metadata integrity check failure")
		}

		Debug("Metadata tarball for the repository " + r.GetName() + " downloaded correctly.")

		if r.Cached {
			// Copy updated repository.yaml file to repo dir now that the tree is synced.
			err = helpers.CopyFile(file, filepath.Join(repobasedir, REPOSITORY_SPECFILE))
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

		tsec, _ := strconv.ParseInt(repo.GetLastUpdate(), 10, 64)

		InfoC(
			aurora.Bold(
				aurora.Red(":house: Repository "+repo.GetName()+" revision: ")).String() +
				aurora.Bold(aurora.Green(repo.GetRevision())).String() + " - " +
				aurora.Bold(aurora.Green(time.Unix(tsec, 0).String())).String(),
		)

	} else {
		Info("Repository", repo.GetName(), "is already up to date.")
	}

	meta, err := NewLuetSystemRepositoryMetadata(
		filepath.Join(metafs, REPOSITORY_METAFILE), false,
	)
	if err != nil {
		return nil, errors.Wrap(err, "While processing "+REPOSITORY_METAFILE)
	}
	repo.SetIndex(meta.ToArtifactIndex())

	reciper := tree.NewInstallerRecipe(pkg.NewInMemoryDatabase(false))
	err = reciper.Load(treefs)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while unpacking rootfs")
	}

	repo.SetTree(reciper)
	repo.SetTreePath(treefs)

	// Copy the local available data to the one which was synced
	// e.g. locally we can override the type (disk), or priority
	// while remotely it could be advertized differently
	repo.SetUrls(r.GetUrls())
	repo.SetAuthentication(r.GetAuthentication())
	repo.SetType(r.GetType())
	repo.SetPriority(r.GetPriority())
	repo.SetName(r.GetName())
	InfoC(
		aurora.Yellow(":information_source:").String() +
			aurora.Magenta("Repository: ").String() +
			aurora.Green(aurora.Bold(repo.GetName()).String()).String() +
			aurora.Magenta(" Priority: ").String() +
			aurora.Bold(aurora.Green(repo.GetPriority())).String() +
			aurora.Magenta(" Type: ").String() +
			aurora.Bold(aurora.Green(repo.GetType())).String(),
	)
	return repo, nil
}

func (r *LuetSystemRepository) Serialize() (*LuetSystemRepositoryMetadata, LuetSystemRepositorySerialized) {

	serialized := LuetSystemRepositorySerialized{
		Name:            r.Name,
		Description:     r.Description,
		Urls:            r.Urls,
		Priority:        r.Priority,
		Type:            r.Type,
		Revision:        r.Revision,
		LastUpdate:      r.LastUpdate,
		RepositoryFiles: r.RepositoryFiles,
	}

	// Check if is needed set the index or simply use
	// value returned by CleanPath
	r.Index = r.Index.CleanPath()

	meta := &LuetSystemRepositoryMetadata{
		Index: []*compiler.PackageArtifact{},
	}
	for _, a := range r.Index {
		art := a.(*compiler.PackageArtifact)
		meta.Index = append(meta.Index, art)
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
	Repo    Repository
	Package pkg.Package
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

func (re Repositories) SearchPackages(p string, o LuetSearchOpts) []PackageMatch {
	sort.Sort(re)
	var matches []PackageMatch
	var err error

	for _, r := range re {
		var repoMatches pkg.Packages

		switch o.Mode {
		case SRegexPkg:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageMatch(p)
		case SLabel:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageLabel(p)
		case SRegexLabel:
			repoMatches, err = r.GetTree().GetDatabase().FindPackageLabelMatch(p)
		}

		if err == nil && len(repoMatches) > 0 {
			for _, pack := range repoMatches {
				matches = append(matches, PackageMatch{Package: pack, Repo: r})
			}
		}
	}

	return matches
}

func (re Repositories) SearchLabelMatch(s string) []PackageMatch {
	return re.SearchPackages(s, LuetSearchOpts{Pattern: s, Mode: SRegexLabel})
}

func (re Repositories) SearchLabel(s string) []PackageMatch {
	return re.SearchPackages(s, LuetSearchOpts{Pattern: s, Mode: SLabel})
}

func (re Repositories) Search(s string) []PackageMatch {
	return re.SearchPackages(s, LuetSearchOpts{Pattern: s, Mode: SRegexPkg})
}
