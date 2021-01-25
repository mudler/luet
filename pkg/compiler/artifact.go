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

package compiler

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"

	system "github.com/docker/docker/pkg/system"
	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"

	//"strconv"
	"strings"
	"sync"

	bus "github.com/mudler/luet/pkg/bus"
	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type CompressionImplementation string

const (
	None      CompressionImplementation = "none" // e.g. tar for standard packages
	GZip      CompressionImplementation = "gzip"
	Zstandard CompressionImplementation = "zstd"
)

type ArtifactIndex []Artifact

func (i ArtifactIndex) CleanPath() ArtifactIndex {
	newIndex := ArtifactIndex{}
	for _, n := range i {
		art := n.(*PackageArtifact)
		// FIXME: This is a dup and makes difficult to add attributes to artifacts
		newIndex = append(newIndex, &PackageArtifact{
			Path:            path.Base(n.GetPath()),
			SourceAssertion: art.SourceAssertion,
			CompileSpec:     art.CompileSpec,
			Dependencies:    art.Dependencies,
			CompressionType: art.CompressionType,
			Checksums:       art.Checksums,
			Files:           art.Files,
		})
	}
	return newIndex
	//Update if exists, otherwise just create
}

//  When compiling, we write also a fingerprint.metadata.yaml file with PackageArtifact. In this way we can have another command to create the repository
// which will consist in just of an repository.yaml which is just the repository structure with the list of package artifact.
// In this way a generic client can fetch the packages and, after unpacking the tree, performing queries to install packages.
type PackageArtifact struct {
	Path string `json:"path"`

	Dependencies    []*PackageArtifact        `json:"dependencies"`
	CompileSpec     *LuetCompilationSpec      `json:"compilationspec"`
	Checksums       Checksums                 `json:"checksums"`
	SourceAssertion solver.PackagesAssertions `json:"-"`
	CompressionType CompressionImplementation `json:"compressiontype"`
	Files           []string                  `json:"files"`
}

func NewPackageArtifact(path string) Artifact {
	return &PackageArtifact{Path: path, Dependencies: []*PackageArtifact{}, Checksums: Checksums{}, CompressionType: None}
}

func NewPackageArtifactFromYaml(data []byte) (Artifact, error) {
	p := &PackageArtifact{Checksums: Checksums{}}
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return p, err
	}

	return p, err
}

func LoadArtifactFromYaml(spec CompilationSpec) (Artifact, error) {

	metaFile := spec.GetPackage().GetFingerPrint() + ".metadata.yaml"
	dat, err := ioutil.ReadFile(spec.Rel(metaFile))
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+metaFile)
	}
	art, err := NewPackageArtifactFromYaml(dat)
	if err != nil {
		return nil, errors.Wrap(err, "Error writing file "+metaFile)
	}
	// It is relative, set it back to abs
	art.SetPath(spec.Rel(art.GetPath()))
	return art, nil
}

func (a *PackageArtifact) SetCompressionType(t CompressionImplementation) {
	a.CompressionType = t
}

func (a *PackageArtifact) GetChecksums() Checksums {
	return a.Checksums
}

func (a *PackageArtifact) SetChecksums(c Checksums) {
	a.Checksums = c
}

func (a *PackageArtifact) SetFiles(f []string) {
	a.Files = f
}

func (a *PackageArtifact) GetFiles() []string {
	return a.Files
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

	mangle.GetCompileSpec().GetPackage().SetPath("")
	for _, ass := range mangle.GetCompileSpec().GetSourceAssertion() {
		ass.Package.SetPath("")
	}

	data, err = yaml.Marshal(mangle)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	err = ioutil.WriteFile(filepath.Join(dst, a.GetCompileSpec().GetPackage().GetFingerPrint()+".metadata.yaml"), data, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "While writing PackageArtifact YAML")
	}
	//a.CompileSpec.GetPackage().SetPath(p)
	bus.Manager.Publish(bus.EventPackagePostBuildArtifact, a)

	return nil
}

func (a *PackageArtifact) GetSourceAssertion() solver.PackagesAssertions {
	return a.SourceAssertion
}

func (a *PackageArtifact) SetCompileSpec(as CompilationSpec) {
	a.CompileSpec = as.(*LuetCompilationSpec)
}

func (a *PackageArtifact) GetCompileSpec() CompilationSpec {
	return a.CompileSpec
}

func (a *PackageArtifact) SetSourceAssertion(as solver.PackagesAssertions) {
	a.SourceAssertion = as
}

func (a *PackageArtifact) GetDependencies() []Artifact {
	ret := []Artifact{}
	for _, d := range a.Dependencies {
		ret = append(ret, d)
	}
	return ret
}

func (a *PackageArtifact) SetDependencies(d []Artifact) {
	ret := []*PackageArtifact{}
	for _, dd := range d {
		ret = append(ret, dd.(*PackageArtifact))
	}
	a.Dependencies = ret
}

func (a *PackageArtifact) GetPath() string {
	return a.Path
}

func (a *PackageArtifact) GetFileName() string {
	return path.Base(a.GetPath())
}

func (a *PackageArtifact) SetPath(p string) {
	a.Path = p
}

func (a *PackageArtifact) genDockerfile() string {
	return `
FROM scratch
COPY * /`
}

// CreateArtifactForFile creates a new artifact from the given file
func CreateArtifactForFile(s string, opts ...func(*PackageArtifact)) (*PackageArtifact, error) {

	fileName := path.Base(s)
	archive, err := LuetCfg.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	defer os.RemoveAll(archive) // clean up
	helpers.CopyFile(s, filepath.Join(archive, fileName))
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

// GenerateFinalImage takes an artifact and builds a Docker image with its content
func (a *PackageArtifact) GenerateFinalImage(imageName string, b CompilerBackend, keepPerms bool) (CompilerBackendOptions, error) {
	builderOpts := CompilerBackendOptions{}
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

	data := a.genDockerfile()
	if err := ioutil.WriteFile(dockerFile, []byte(data), 0644); err != nil {
		return builderOpts, errors.Wrap(err, "error met while rendering artifact dockerfile "+a.Path)
	}

	if err := a.Unpack(uncompressedFiles, keepPerms); err != nil {
		return builderOpts, errors.Wrap(err, "error met while uncompressing artifact "+a.Path)
	}

	builderOpts = CompilerBackendOptions{
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

	case Zstandard:
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
	case GZip:
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
	return errors.New("Compression type must be supplied")
}

func (a *PackageArtifact) getCompressedName() string {
	switch a.CompressionType {
	case Zstandard:
		return a.Path + ".zst"

	case GZip:
		return a.Path + ".gz"
	}
	return a.Path
}

// GetUncompressedName returns the artifact path without the extension suffix
func (a *PackageArtifact) GetUncompressedName() string {
	switch a.CompressionType {
	case Zstandard, GZip:
		return strings.TrimSuffix(a.Path, filepath.Ext(a.Path))
	}
	return a.Path
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

		// Check if exists
		if helpers.Exists(destPath) {
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
	case Zstandard:
		// Create the uncompressed archive
		archive, err := os.Create(a.GetPath() + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.GetPath() + ".uncompressed")
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
			return errors.Wrap(err, "Cannot copy to "+a.GetPath()+".uncompressed")
		}

		err = helpers.UntarProtect(a.GetPath()+".uncompressed", dst,
			LuetCfg.GetGeneral().SameOwner, protectedFiles, tarModifier)
		if err != nil {
			return err
		}
		return nil
	case GZip:
		// Create the uncompressed archive
		archive, err := os.Create(a.GetPath() + ".uncompressed")
		if err != nil {
			return err
		}
		defer os.RemoveAll(a.GetPath() + ".uncompressed")
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
			return errors.Wrap(err, "Cannot copy to "+a.GetPath()+".uncompressed")
		}

		err = helpers.UntarProtect(a.GetPath()+".uncompressed", dst,
			LuetCfg.GetGeneral().SameOwner, protectedFiles, tarModifier)
		if err != nil {
			return err
		}
		return nil
	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.UntarProtect(a.GetPath(), dst, LuetCfg.GetGeneral().SameOwner,
			protectedFiles, tarModifier)
	}
	return errors.New("Compression type must be supplied")
}

// FileList generates the list of file of a package from the local archive
func (a *PackageArtifact) FileList() ([]string, error) {
	var tr *tar.Reader
	switch a.CompressionType {
	case Zstandard:
		archive, err := os.Create(a.GetPath() + ".uncompressed")
		if err != nil {
			return []string{}, err
		}
		defer os.RemoveAll(a.GetPath() + ".uncompressed")
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
	case GZip:
		// Create the uncompressed archive
		archive, err := os.Create(a.GetPath() + ".uncompressed")
		if err != nil {
			return []string{}, err
		}
		defer os.RemoveAll(a.GetPath() + ".uncompressed")
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
		tarFile, err := os.Open(a.GetPath())
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

func copyXattr(srcPath, dstPath, attr string) error {
	data, err := system.Lgetxattr(srcPath, attr)
	if err != nil {
		return err
	}
	if data != nil {
		if err := system.Lsetxattr(dstPath, attr, data, 0); err != nil {
			return err
		}
	}
	return nil
}

func doCopyXattrs(srcPath, dstPath string) error {
	if err := copyXattr(srcPath, dstPath, "security.capability"); err != nil {
		return err
	}

	return copyXattr(srcPath, dstPath, "trusted.overlay.opaque")
}

func worker(i int, wg *sync.WaitGroup, s <-chan CopyJob) {
	defer wg.Done()

	for job := range s {
		//Info("#"+strconv.Itoa(i), "copying", job.Src, "to", job.Dst)
		// if dir, err := helpers.IsDirectory(job.Src); err == nil && dir {
		// 	err = helpers.CopyDir(job.Src, job.Dst)
		// 	if err != nil {
		// 		Warning("Error copying dir", job, err)
		// 	}
		// 	continue
		// }

		_, err := os.Lstat(job.Dst)
		if err != nil {
			Debug("Copying ", job.Src)
			if err := helpers.CopyFile(job.Src, job.Dst); err != nil {
				Warning("Error copying", job, err)
			}
			doCopyXattrs(job.Src, job.Dst)
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

// ExtractArtifactFromDelta extracts deltas from ArtifactLayer from an image in tar format
func ExtractArtifactFromDelta(src, dst string, layers []ArtifactLayer, concurrency int, keepPerms bool, includes []string, excludes []string, t CompressionImplementation) (Artifact, error) {

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
	a.SetCompressionType(t)
	err = a.Compress(archive, concurrency)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}
	return a, nil
}

func ComputeArtifactLayerSummary(diffs []ArtifactLayer) ArtifactLayersSummary {

	ans := ArtifactLayersSummary{
		Layers: make([]ArtifactLayerSummary, 0),
	}

	for _, layer := range diffs {
		sum := ArtifactLayerSummary{
			FromImage:   layer.FromImage,
			ToImage:     layer.ToImage,
			AddFiles:    0,
			AddSizes:    0,
			DelFiles:    0,
			DelSizes:    0,
			ChangeFiles: 0,
			ChangeSizes: 0,
		}
		for _, a := range layer.Diffs.Additions {
			sum.AddFiles++
			sum.AddSizes += int64(a.Size)
		}
		for _, d := range layer.Diffs.Deletions {
			sum.DelFiles++
			sum.DelSizes += int64(d.Size)
		}
		for _, c := range layer.Diffs.Changes {
			sum.ChangeFiles++
			sum.ChangeSizes += int64(c.Size)
		}
		ans.Layers = append(ans.Layers, sum)
	}

	return ans
}
