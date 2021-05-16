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

package artifact

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"

	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"

	//"strconv"
	"strings"
	"sync"

	bus "github.com/mudler/luet/pkg/bus"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	compression "github.com/mudler/luet/pkg/compiler/types/compression"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

//  When compiling, we write also a fingerprint.metadata.yaml file with PackageArtifact. In this way we can have another command to create the repository
// which will consist in just of an repository.yaml which is just the repository structure with the list of package artifact.
// In this way a generic client can fetch the packages and, after unpacking the tree, performing queries to install packages.
type PackageArtifact struct {
	Path string `json:"path"`

	Dependencies    []*PackageArtifact                `json:"dependencies"`
	CompileSpec     *compilerspec.LuetCompilationSpec `json:"compilationspec"`
	Checksums       Checksums                         `json:"checksums"`
	SourceAssertion solver.PackagesAssertions         `json:"-"`
	CompressionType compression.Implementation        `json:"compressiontype"`
	Files           []string                          `json:"files"`
}

func (p *PackageArtifact) ShallowCopy() *PackageArtifact {
	copy := *p
	return &copy
}

func NewPackageArtifact(path string) *PackageArtifact {
	return &PackageArtifact{Path: path, Dependencies: []*PackageArtifact{}, Checksums: Checksums{}, CompressionType: compression.None}
}

func NewPackageArtifactFromYaml(data []byte) (*PackageArtifact, error) {
	p := &PackageArtifact{Checksums: Checksums{}}
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return p, err
	}

	return p, err
}

func (a *PackageArtifact) Hash() error {
	return a.Checksums.Generate(a)
}

func (a *PackageArtifact) Verify() error {
	sum := Checksums{}
	if err := sum.Generate(a); err != nil {
		return err
	}

	if err := sum.Compare(a.Checksums); err != nil {
		return err
	}

	return nil
}

func (a *PackageArtifact) WriteYaml(dst string) error {
	// First compute checksum of artifact. When we write the yaml we want to write up-to-date informations.
	err := a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for artifact")
	}

	//p := a.CompileSpec.GetPackage().GetPath()

	//a.CompileSpec.GetPackage().SetPath("")
	//	for _, ass := range a.CompileSpec.GetSourceAssertion() {
	//		ass.Package.SetPath("")
	//	}
	data, err := yaml.Marshal(a)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	bus.Manager.Publish(bus.EventPackagePreBuildArtifact, a)

	mangle, err := NewPackageArtifactFromYaml(data)
	if err != nil {
		return errors.Wrap(err, "Generated invalid artifact")
	}
	//p := a.CompileSpec.GetPackage().GetPath()

	mangle.CompileSpec.GetPackage().SetPath("")
	for _, ass := range mangle.CompileSpec.GetSourceAssertion() {
		ass.Package.SetPath("")
	}

	data, err = yaml.Marshal(mangle)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	err = ioutil.WriteFile(filepath.Join(dst, a.CompileSpec.GetPackage().GetMetadataFilePath()), data, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "While writing PackageArtifact YAML")
	}
	//a.CompileSpec.GetPackage().SetPath(p)
	bus.Manager.Publish(bus.EventPackagePostBuildArtifact, a)

	return nil
}

func (a *PackageArtifact) GetFileName() string {
	return path.Base(a.Path)
}

func (a *PackageArtifact) genDockerfile() string {
	return `
FROM scratch
COPY . /`
}

// CreateArtifactForFile creates a new artifact from the given file
func CreateArtifactForFile(s string, opts ...func(*PackageArtifact)) (*PackageArtifact, error) {
	if _, err := os.Stat(s); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "artifact path doesn't exist")
	}
	fileName := path.Base(s)
	archive, err := LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	defer os.RemoveAll(archive) // clean up
	dst := filepath.Join(archive, fileName)
	if err := helpers.CopyFile(s, dst); err != nil {
		return nil, errors.Wrapf(err, "error while copying %s to %s", s, dst)
	}

	artifact, err := LuetCfg.GetSystem().TempDir("artifact")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	a := &PackageArtifact{Path: filepath.Join(artifact, fileName)}

	for _, o := range opts {
		o(a)
	}

	return a, a.Compress(archive, 1)
}

type ImageBuilder interface {
	BuildImage(backend.Options) error
}

// GenerateFinalImage takes an artifact and builds a Docker image with its content
func (a *PackageArtifact) GenerateFinalImage(imageName string, b ImageBuilder, keepPerms bool) (backend.Options, error) {
	builderOpts := backend.Options{}
	archive, err := LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}
	defer os.RemoveAll(archive) // clean up

	uncompressedFiles := filepath.Join(archive, "files")
	dockerFile := filepath.Join(archive, "Dockerfile")

	if err := os.MkdirAll(uncompressedFiles, os.ModePerm); err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}

	if err := a.Unpack(uncompressedFiles, keepPerms); err != nil {
		return builderOpts, errors.Wrap(err, "error met while uncompressing artifact "+a.Path)
	}

	empty, err := helpers.DirectoryIsEmpty(uncompressedFiles)
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while checking if directory is empty "+uncompressedFiles)
	}

	// See https://github.com/moby/moby/issues/38039.
	// We can't generate FROM scratch empty images. Docker will refuse to export them
	// workaround: Inject a .virtual empty file
	if empty {
		helpers.Touch(filepath.Join(uncompressedFiles, ".virtual"))
	}

	data := a.genDockerfile()
	if err := ioutil.WriteFile(dockerFile, []byte(data), 0644); err != nil {
		return builderOpts, errors.Wrap(err, "error met while rendering artifact dockerfile "+a.Path)
	}

	builderOpts = backend.Options{
		ImageName:      imageName,
		SourcePath:     archive,
		DockerFileName: dockerFile,
		Context:        uncompressedFiles,
	}
	return builderOpts, b.BuildImage(builderOpts)
}

// Compress is responsible to archive and compress to the artifact Path.
// It accepts a source path, which is the content to be archived/compressed
// and a concurrency parameter.
func (a *PackageArtifact) Compress(src string, concurrency int) error {
	switch a.CompressionType {

	case compression.Zstandard:
		err := helpers.Tar(src, a.Path)
		if err != nil {
			return err
		}
		original, err := os.Open(a.Path)
		if err != nil {
			return err
		}
		defer original.Close()

		zstdFile := a.getCompressedName()
		bufferedReader := bufio.NewReader(original)

		// Open a file for writing.
		dst, err := os.Create(zstdFile)
		if err != nil {
			return err
		}

		enc, err := zstd.NewWriter(dst)
		if err != nil {
			return err
		}
		_, err = io.Copy(enc, bufferedReader)
		if err != nil {
			enc.Close()
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}

		os.RemoveAll(a.Path) // Remove original
		Debug("Removed artifact", a.Path)

		a.Path = zstdFile
		return nil
	case compression.GZip:
		err := helpers.Tar(src, a.Path)
		if err != nil {
			return err
		}
		original, err := os.Open(a.Path)
		if err != nil {
			return err
		}
		defer original.Close()

		gzipfile := a.getCompressedName()
		bufferedReader := bufio.NewReader(original)

		// Open a file for writing.
		dst, err := os.Create(gzipfile)
		if err != nil {
			return err
		}
		// Create gzip writer.
		w := gzip.NewWriter(dst)
		w.SetConcurrency(1<<20, concurrency)
		defer w.Close()
		defer dst.Close()
		_, err = io.Copy(w, bufferedReader)
		if err != nil {
			return err
		}
		w.Close()
		os.RemoveAll(a.Path) // Remove original
		Debug("Removed artifact", a.Path)
		//	a.CompressedPath = gzipfile
		a.Path = gzipfile
		return nil
		//a.Path = gzipfile

	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.Tar(src, a.getCompressedName())
	}
}

func (a *PackageArtifact) getCompressedName() string {
	switch a.CompressionType {
	case compression.Zstandard:
		return a.Path + ".zst"

	case compression.GZip:
		return a.Path + ".gz"
	}
	return a.Path
}

// GetUncompressedName returns the artifact path without the extension suffix
func (a *PackageArtifact) GetUncompressedName() string {
	switch a.CompressionType {
	case compression.Zstandard, compression.GZip:
		return strings.TrimSuffix(a.Path, filepath.Ext(a.Path))
	}
	return a.Path
}

func hashContent(bv []byte) string {
	hasher := sha1.New()
	hasher.Write(bv)
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return sha
}

func hashFileContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

func tarModifierWrapperFunc(dst, path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
	// If the destination path already exists I rename target file name with postfix.
	var destPath string

	// Read data. TODO: We need change archive callback to permit to return a Reader
	buffer := bytes.Buffer{}
	if content != nil {
		if _, err := buffer.ReadFrom(content); err != nil {
			return nil, nil, err
		}
	}
	tarHash := hashContent(buffer.Bytes())

	// If file is not present on archive but is defined on mods
	// I receive the callback. Prevent nil exception.
	if header != nil {
		switch header.Typeflag {
		case tar.TypeReg:
			destPath = filepath.Join(dst, path)
		default:
			// Nothing to do. I return original reader
			return header, buffer.Bytes(), nil
		}

		existingHash := ""
		f, err := os.Lstat(destPath)
		if err == nil {
			Debug("File exists already, computing hash for", destPath)
			hash, err := hashFileContent(destPath)
			if err == nil {
				existingHash = hash
			}
		}

		Debug("Existing file hash: ", existingHash, "Tar file hashsum: ", tarHash)
		// We want to protect file only if the hash of the files are differing OR the file size are
		differs := (existingHash != "" && existingHash != tarHash) || header.Size != f.Size()
		// Check if exists
		if helpers.Exists(destPath) && differs {
			for i := 1; i < 1000; i++ {
				name := filepath.Join(filepath.Join(filepath.Dir(path),
					fmt.Sprintf("._cfg%04d_%s", i, filepath.Base(path))))

				if helpers.Exists(name) {
					continue
				}
				Info(fmt.Sprintf("Found protected file %s. Creating %s.", destPath,
					filepath.Join(dst, name)))
				return &tar.Header{
					Mode:       header.Mode,
					Typeflag:   header.Typeflag,
					PAXRecords: header.PAXRecords,
					Name:       name,
				}, buffer.Bytes(), nil
			}
		}
	}

	return header, buffer.Bytes(), nil
}

func (a *PackageArtifact) GetProtectFiles() []string {
	ans := []string{}
	annotationDir := ""

	if !LuetCfg.ConfigProtectSkip {

		// a.CompileSpec could be nil when artifact.Unpack is used for tree tarball
		if a.CompileSpec != nil &&
			a.CompileSpec.GetPackage().HasAnnotation(string(pkg.ConfigProtectAnnnotation)) {
			dir, ok := a.CompileSpec.GetPackage().GetAnnotations()[string(pkg.ConfigProtectAnnnotation)]
			if ok {
				annotationDir = dir
			}
		}
		// TODO: check if skip this if we have a.CompileSpec nil

		cp := NewConfigProtect(annotationDir)
		cp.Map(a.Files)

		// NOTE: for unpack we need files path without initial /
		ans = cp.GetProtectFiles(false)
	}

	return ans
}

// Unpack Untar and decompress (TODO) to the given path
func (a *PackageArtifact) Unpack(dst string, keepPerms bool) error {

	// Create
	protectedFiles := a.GetProtectFiles()

	tarModifier := helpers.NewTarModifierWrapper(dst, tarModifierWrapperFunc)

	switch a.CompressionType {
	case compression.Zstandard:
		// Create the uncompressed archive
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)

		d, err := zstd.NewReader(bufferedReader)
		if err != nil {
			return err
		}
		defer d.Close()

		_, err = io.Copy(archive, d)
		if err != nil {
			return errors.Wrap(err, "Cannot copy to "+a.Path+".uncompressed")
		}

		err = helpers.UntarProtect(a.Path+".uncompressed", dst,
			LuetCfg.GetGeneral().SameOwner, protectedFiles, tarModifier)
		if err != nil {
			return err
		}
		return nil
	case compression.GZip:
		// Create the uncompressed archive
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := gzip.NewReader(bufferedReader)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(archive, r)
		if err != nil {
			return errors.Wrap(err, "Cannot copy to "+a.Path+".uncompressed")
		}

		err = helpers.UntarProtect(a.Path+".uncompressed", dst,
			LuetCfg.GetGeneral().SameOwner, protectedFiles, tarModifier)
		if err != nil {
			return err
		}
		return nil
	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.UntarProtect(a.Path, dst, LuetCfg.GetGeneral().SameOwner,
			protectedFiles, tarModifier)
	}
	return errors.New("Compression type must be supplied")
}

// FileList generates the list of file of a package from the local archive
func (a *PackageArtifact) FileList() ([]string, error) {
	var tr *tar.Reader
	switch a.CompressionType {
	case compression.Zstandard:
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return []string{}, err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := zstd.NewReader(bufferedReader)
		if err != nil {
			return []string{}, err
		}
		defer r.Close()
		tr = tar.NewReader(r)
	case compression.GZip:
		// Create the uncompressed archive
		archive, err := os.Create(a.Path + ".uncompressed")
		if err != nil {
			return []string{}, err
		}
		defer os.RemoveAll(a.Path + ".uncompressed")
		defer archive.Close()

		original, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Cannot open "+a.Path)
		}
		defer original.Close()

		bufferedReader := bufio.NewReader(original)
		r, err := gzip.NewReader(bufferedReader)
		if err != nil {
			return []string{}, err
		}
		defer r.Close()
		tr = tar.NewReader(r)

	// Defaults to tar only (covers when "none" is supplied)
	default:
		tarFile, err := os.Open(a.Path)
		if err != nil {
			return []string{}, errors.Wrap(err, "Could not open package archive")
		}
		defer tarFile.Close()
		tr = tar.NewReader(tarFile)

	}

	var files []string
	// untar each segment
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []string{}, err
		}
		// determine proper file path info
		finfo := hdr.FileInfo()
		fileName := hdr.Name
		if finfo.Mode().IsDir() {
			continue
		}
		files = append(files, fileName)

		// if a dir, create it, then go to next segment
	}
	return files, nil
}

type CopyJob struct {
	Src, Dst string
	Artifact string
}

func worker(i int, wg *sync.WaitGroup, s <-chan CopyJob) {
	defer wg.Done()

	for job := range s {
		_, err := os.Lstat(job.Dst)
		if err != nil {
			Debug("Copying ", job.Src)
			if err := helpers.DeepCopyFile(job.Src, job.Dst); err != nil {
				Warning("Error copying", job, err)
			}
		}
	}
}

func compileRegexes(regexes []string) []*regexp.Regexp {
	var result []*regexp.Regexp
	for _, i := range regexes {
		r, e := regexp.Compile(i)
		if e != nil {
			Warning("Failed compiling regex:", e)
			continue
		}
		result = append(result, r)
	}
	return result
}

type ArtifactNode struct {
	Name string `json:"Name"`
	Size int    `json:"Size"`
}
type ArtifactDiffs struct {
	Additions []ArtifactNode `json:"Adds"`
	Deletions []ArtifactNode `json:"Dels"`
	Changes   []ArtifactNode `json:"Mods"`
}

type ArtifactLayer struct {
	FromImage string        `json:"Image1"`
	ToImage   string        `json:"Image2"`
	Diffs     ArtifactDiffs `json:"Diff"`
}

// ExtractArtifactFromDelta extracts deltas from ArtifactLayer from an image in tar format
func ExtractArtifactFromDelta(src, dst string, layers []ArtifactLayer, concurrency int, keepPerms bool, includes []string, excludes []string, t compression.Implementation) (*PackageArtifact, error) {

	archive, err := LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up

	if strings.HasSuffix(src, ".tar") {
		rootfs, err := LuetCfg.GetSystem().TempDir("rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
		defer os.RemoveAll(rootfs) // clean up
		err = helpers.Untar(src, rootfs, keepPerms)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking rootfs")
		}
		src = rootfs
	}

	toCopy := make(chan CopyJob)

	var wg = new(sync.WaitGroup)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i, wg, toCopy)
	}

	// Handle includes in spec. If specified they filter what gets in the package

	if len(includes) > 0 && len(excludes) == 0 {
		includeRegexp := compileRegexes(includes)
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		ADDS:
			for _, a := range l.Diffs.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) {
						toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
						continue ADDS
					}
				}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else if len(includes) == 0 && len(excludes) != 0 {
		excludeRegexp := compileRegexes(excludes)
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		ADD:
			for _, a := range l.Diffs.Additions {
				for _, i := range excludeRegexp {
					if i.MatchString(a.Name) {
						continue ADD
					}
				}
				toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else if len(includes) != 0 && len(excludes) != 0 {
		includeRegexp := compileRegexes(includes)
		excludeRegexp := compileRegexes(excludes)

		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		EXCLUDES:
			for _, a := range l.Diffs.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) {
						for _, e := range excludeRegexp {
							if e.MatchString(a.Name) {
								continue EXCLUDES
							}
						}
						toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
						continue EXCLUDES
					}
				}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}

	} else {
		// Otherwise just grab all
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
			for _, a := range l.Diffs.Additions {
				Debug("File ", a.Name, " added")
				toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
			}
			for _, a := range l.Diffs.Changes {
				Debug("File ", a.Name, " changed")
			}
			for _, a := range l.Diffs.Deletions {
				Debug("File ", a.Name, " deleted")
			}
		}
	}

	close(toCopy)
	wg.Wait()

	a := NewPackageArtifact(dst)
	a.CompressionType = t
	err = a.Compress(archive, concurrency)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}
	return a, nil
}
