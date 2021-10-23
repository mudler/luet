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

	v1 "github.com/google/go-containerregistry/pkg/v1"
	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"

	//"strconv"
	"strings"

	config "github.com/mudler/luet/pkg/api/core/config"
	"github.com/mudler/luet/pkg/api/core/image"
	types "github.com/mudler/luet/pkg/api/core/types"
	bus "github.com/mudler/luet/pkg/bus"
	backend "github.com/mudler/luet/pkg/compiler/backend"
	compression "github.com/mudler/luet/pkg/compiler/types/compression"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// When compiling, we write also a fingerprint.metadata.yaml file with PackageArtifact. In this way we can have another command to create the repository
// which will consist in just of an repository.yaml which is just the repository structure with the list of package artifact.
// In this way a generic client can fetch the packages and, after unpacking the tree, performing queries to install packages.
type PackageArtifact struct {
	Path string `json:"path"`

	Dependencies      []*PackageArtifact                `json:"dependencies"`
	CompileSpec       *compilerspec.LuetCompilationSpec `json:"compilationspec"`
	Checksums         Checksums                         `json:"checksums"`
	SourceAssertion   solver.PackagesAssertions         `json:"-"`
	CompressionType   compression.Implementation        `json:"compressiontype"`
	Files             []string                          `json:"files"`
	PackageCacheImage string                            `json:"package_cacheimage"`
	Runtime           *pkg.DefaultPackage               `json:"runtime,omitempty"`
}

func ImageToArtifact(ctx *types.Context, img v1.Image, t compression.Implementation, output string, filter func(h *tar.Header) (bool, error)) (*PackageArtifact, error) {
	_, tmpdiffs, err := image.Extract(ctx, img, filter)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(tmpdiffs) // clean up

	a := NewPackageArtifact(output)
	a.CompressionType = t
	err = a.Compress(tmpdiffs, 1)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}
	return a, nil
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

func (a *PackageArtifact) WriteYAML(dst string) error {
	// First compute checksum of artifact. When we write the yaml we want to write up-to-date informations.
	err := a.Hash()
	if err != nil {
		return errors.Wrap(err, "Failed generating checksums for artifact")
	}

	// Update runtime package information
	if a.CompileSpec != nil && a.CompileSpec.Package != nil {
		runtime, err := a.CompileSpec.Package.GetRuntimePackage()
		if err != nil {
			return errors.Wrapf(err, "getting runtime package for '%s'", a.CompileSpec.Package.HumanReadableString())
		}
		a.Runtime = runtime
	}

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
func CreateArtifactForFile(ctx *types.Context, s string, opts ...func(*PackageArtifact)) (*PackageArtifact, error) {
	if _, err := os.Stat(s); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "artifact path doesn't exist")
	}
	fileName := path.Base(s)
	archive, err := ctx.Config.GetSystem().TempDir("archive")
	if err != nil {
		return nil, errors.Wrap(err, "error met while creating tempdir for "+s)
	}
	defer os.RemoveAll(archive) // clean up
	dst := filepath.Join(archive, fileName)
	if err := fileHelper.CopyFile(s, dst); err != nil {
		return nil, errors.Wrapf(err, "error while copying %s to %s", s, dst)
	}

	artifact, err := ctx.Config.GetSystem().TempDir("artifact")
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
func (a *PackageArtifact) GenerateFinalImage(ctx *types.Context, imageName string, b ImageBuilder, keepPerms bool) (backend.Options, error) {
	builderOpts := backend.Options{}
	archive, err := ctx.Config.GetSystem().TempDir("archive")
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}
	defer os.RemoveAll(archive) // clean up

	uncompressedFiles := filepath.Join(archive, "files")
	dockerFile := filepath.Join(archive, "Dockerfile")

	if err := os.MkdirAll(uncompressedFiles, os.ModePerm); err != nil {
		return builderOpts, errors.Wrap(err, "error met while creating tempdir for "+a.Path)
	}

	if err := a.Unpack(ctx, uncompressedFiles, keepPerms); err != nil {
		return builderOpts, errors.Wrap(err, "error met while uncompressing artifact "+a.Path)
	}

	empty, err := fileHelper.DirectoryIsEmpty(uncompressedFiles)
	if err != nil {
		return builderOpts, errors.Wrap(err, "error met while checking if directory is empty "+uncompressedFiles)
	}

	// See https://github.com/moby/moby/issues/38039.
	// We can't generate FROM scratch empty images. Docker will refuse to export them
	// workaround: Inject a .virtual empty file
	if empty {
		fileHelper.Touch(filepath.Join(uncompressedFiles, ".virtual"))
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
		//	Debug("Removed artifact", a.Path)

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
		//	Debug("Removed artifact", a.Path)
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

func tarModifierWrapperFunc(ctx *types.Context) func(dst, path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
	return func(dst, path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
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
				ctx.Debug("File exists already, computing hash for", destPath)
				hash, herr := hashFileContent(destPath)
				if herr == nil {
					existingHash = hash
				}
			}

			ctx.Debug("Existing file hash: ", existingHash, "Tar file hashsum: ", tarHash)
			// We want to protect file only if the hash of the files are differing OR the file size are
			differs := (existingHash != "" && existingHash != tarHash) || (err != nil && f != nil && header.Size != f.Size())
			// Check if exists
			if fileHelper.Exists(destPath) && differs {
				for i := 1; i < 1000; i++ {
					name := filepath.Join(filepath.Join(filepath.Dir(path),
						fmt.Sprintf("._cfg%04d_%s", i, filepath.Base(path))))

					if fileHelper.Exists(name) {
						continue
					}
					ctx.Info(fmt.Sprintf("Found protected file %s. Creating %s.", destPath,
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
}

func (a *PackageArtifact) GetProtectFiles(ctx *types.Context) []string {
	ans := []string{}
	annotationDir := ""

	if !ctx.Config.ConfigProtectSkip {

		// a.CompileSpec could be nil when artifact.Unpack is used for tree tarball
		if a.CompileSpec != nil &&
			a.CompileSpec.GetPackage().HasAnnotation(string(pkg.ConfigProtectAnnnotation)) {
			dir, ok := a.CompileSpec.GetPackage().GetAnnotations()[string(pkg.ConfigProtectAnnnotation)]
			if ok {
				annotationDir = dir
			}
		}
		// TODO: check if skip this if we have a.CompileSpec nil

		cp := config.NewConfigProtect(annotationDir)
		cp.Map(a.Files, ctx.Config.GetConfigProtectConfFiles())

		// NOTE: for unpack we need files path without initial /
		ans = cp.GetProtectFiles(false)
	}

	return ans
}

// Unpack Untar and decompress (TODO) to the given path
func (a *PackageArtifact) Unpack(ctx *types.Context, dst string, keepPerms bool) error {
	if !strings.HasPrefix(dst, "/") {
		return errors.New("destination must be an absolute path")
	}

	// Create
	protectedFiles := a.GetProtectFiles(ctx)

	tarModifier := helpers.NewTarModifierWrapper(dst, tarModifierWrapperFunc(ctx))

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
			ctx.Config.GetGeneral().SameOwner, protectedFiles, tarModifier)
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
			ctx.Config.GetGeneral().SameOwner, protectedFiles, tarModifier)
		if err != nil {
			return err
		}
		return nil
	// Defaults to tar only (covers when "none" is supplied)
	default:
		return helpers.UntarProtect(a.Path, dst, ctx.Config.GetGeneral().SameOwner,
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
