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

package compiler

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/imdario/mergo"
	bus "github.com/mudler/luet/pkg/bus"
	"github.com/mudler/luet/pkg/compiler/backend"
	artifact "github.com/mudler/luet/pkg/compiler/types/artifact"
	"github.com/mudler/luet/pkg/compiler/types/options"
	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
)

const BuildFile = "build.yaml"
const DefinitionFile = "definition.yaml"
const CollectionFile = "collection.yaml"

type ArtifactIndex []*artifact.PackageArtifact

func (i ArtifactIndex) CleanPath() ArtifactIndex {
	newIndex := ArtifactIndex{}
	for _, art := range i {
		copy := art.ShallowCopy()
		copy.Path = path.Base(art.Path)
		newIndex = append(newIndex, copy)
	}
	return newIndex
}

type LuetCompiler struct {
	//*tree.CompilerRecipe
	Backend  CompilerBackend
	Database pkg.PackageDatabase
	Options  options.Compiler
}

func NewCompiler(p ...options.Option) *LuetCompiler {
	c := options.NewDefaultCompiler()
	c.Apply(p...)

	return &LuetCompiler{Options: *c}
}

func NewLuetCompiler(backend CompilerBackend, db pkg.PackageDatabase, compilerOpts ...options.Option) *LuetCompiler {
	// The CompilerRecipe will gives us a tree with only build deps listed.

	c := NewCompiler(compilerOpts...)
	//	c.Options.BackendType
	c.Backend = backend
	c.Database = db
	// c.CompilerRecipe = &tree.CompilerRecipe{
	// 	Recipe: tree.Recipe{Database: db},
	// }

	return c
}

func (cs *LuetCompiler) compilerWorker(i int, wg *sync.WaitGroup, cspecs chan *compilerspec.LuetCompilationSpec, a *[]*artifact.PackageArtifact, m *sync.Mutex, concurrency int, keepPermissions bool, errors chan error) {
	defer wg.Done()

	for s := range cspecs {
		ar, err := cs.compile(concurrency, keepPermissions, nil, nil, s)
		if err != nil {
			errors <- err
		}

		m.Lock()
		*a = append(*a, ar)
		m.Unlock()
	}
}

// CompileWithReverseDeps compiles the supplied compilationspecs and their reverse dependencies
func (cs *LuetCompiler) CompileWithReverseDeps(keepPermissions bool, ps *compilerspec.LuetCompilationspecs) ([]*artifact.PackageArtifact, []error) {
	artifacts, err := cs.CompileParallel(keepPermissions, ps)
	if len(err) != 0 {
		return artifacts, err
	}

	Info(":ant: Resolving reverse dependencies")
	toCompile := compilerspec.NewLuetCompilationspecs()
	for _, a := range artifacts {

		revdeps := a.CompileSpec.GetPackage().Revdeps(cs.Database)
		for _, r := range revdeps {
			spec, asserterr := cs.FromPackage(r)
			if err != nil {
				return nil, append(err, asserterr)
			}
			spec.SetOutputPath(ps.All()[0].GetOutputPath())

			toCompile.Add(spec)
		}
	}

	uniques := toCompile.Unique().Remove(ps)
	for _, u := range uniques.All() {
		Info(" :arrow_right_hook:", u.GetPackage().GetName(), ":leaves:", u.GetPackage().GetVersion(), "(", u.GetPackage().GetCategory(), ")")
	}

	artifacts2, err := cs.CompileParallel(keepPermissions, uniques)
	return append(artifacts, artifacts2...), err
}

// CompileParallel compiles the supplied compilationspecs in parallel
// to note, no specific heuristic is implemented, and the specs are run in parallel as they are.
func (cs *LuetCompiler) CompileParallel(keepPermissions bool, ps *compilerspec.LuetCompilationspecs) ([]*artifact.PackageArtifact, []error) {
	all := make(chan *compilerspec.LuetCompilationSpec)
	artifacts := []*artifact.PackageArtifact{}
	mutex := &sync.Mutex{}
	errors := make(chan error, ps.Len())
	var wg = new(sync.WaitGroup)
	for i := 0; i < cs.Options.Concurrency; i++ {
		wg.Add(1)
		go cs.compilerWorker(i, wg, all, &artifacts, mutex, cs.Options.Concurrency, keepPermissions, errors)
	}

	for _, p := range ps.All() {
		all <- p
	}

	close(all)
	wg.Wait()
	close(errors)

	var allErrors []error

	for e := range errors {
		allErrors = append(allErrors, e)
	}

	return artifacts, allErrors
}

func (cs *LuetCompiler) stripFromRootfs(includes []string, rootfs string, include bool) error {
	var includeRegexp []*regexp.Regexp
	for _, i := range includes {
		r, e := regexp.Compile(i)
		if e != nil {
			return errors.Wrap(e, "Could not compile regex in the include of the package")
		}
		includeRegexp = append(includeRegexp, r)
	}

	toRemove := []string{}

	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		// if info.Name() != DefinitionFile {
		// 	return nil // Skip with no errors
		// }
		if currentpath == rootfs {
			return nil
		}

		abspath := strings.ReplaceAll(currentpath, rootfs, "")

		match := false

		for _, i := range includeRegexp {
			if i.MatchString(abspath) {
				match = true
				break
			}
		}

		if include && !match || !include && match {
			toRemove = append(toRemove, currentpath)
			Debug(":scissors: Removing file", currentpath)
		} else {
			Debug(":sun: Matched file", currentpath)
		}

		return nil
	}

	err := filepath.Walk(rootfs, ff)
	if err != nil {
		return err
	}

	for _, s := range toRemove {
		e := os.RemoveAll(s)
		if e != nil {
			Warning("Failed removing", s, e.Error())
			return e
		}
	}
	return nil
}

func (cs *LuetCompiler) unpackFs(concurrency int, keepPermissions bool, p *compilerspec.LuetCompilationSpec, runnerOpts backend.Options) (*artifact.PackageArtifact, error) {

	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	err = cs.Backend.ExtractRootfs(backend.Options{
		ImageName: runnerOpts.ImageName, Destination: rootfs}, keepPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}

	if p.GetPackageDir() != "" {
		Info(":tophat: Packing from output dir", p.GetPackageDir())
		rootfs = filepath.Join(rootfs, p.GetPackageDir())
	}

	if len(p.GetIncludes()) > 0 {
		// strip from includes
		cs.stripFromRootfs(p.GetIncludes(), rootfs, true)
	}
	if len(p.GetExcludes()) > 0 {
		// strip from excludes
		cs.stripFromRootfs(p.GetExcludes(), rootfs, false)
	}
	a := artifact.NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar"))
	a.CompressionType = cs.Options.CompressionType

	if err := a.Compress(rootfs, concurrency); err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}

	a.CompileSpec = p
	return a, nil
}

func (cs *LuetCompiler) unpackDelta(concurrency int, keepPermissions bool, p *compilerspec.LuetCompilationSpec, builderOpts, runnerOpts backend.Options) (*artifact.PackageArtifact, error) {

	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	pkgTag := ":package: " + p.GetPackage().HumanReadableString()
	if cs.Options.PullFirst && !cs.Backend.ImageExists(builderOpts.ImageName) && cs.Backend.ImageAvailable(builderOpts.ImageName) {
		err := cs.Backend.DownloadImage(builderOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Could not pull image")
		}
	}

	Info(pkgTag, ":hammer: Generating delta")
	diffs, err := GenerateChanges(cs.Backend, builderOpts, runnerOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate changes from layers")
	}

	Debug("Extracting image to grab files from delta")
	if err := cs.Backend.ExtractRootfs(backend.Options{
		ImageName: runnerOpts.ImageName, Destination: rootfs}, keepPermissions); err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}
	artifact, err := artifact.ExtractArtifactFromDelta(rootfs, p.Rel(p.GetPackage().GetFingerPrint()+".package.tar"), diffs, concurrency, keepPermissions, p.GetIncludes(), p.GetExcludes(), cs.Options.CompressionType)
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate deltas")
	}

	artifact.CompileSpec = p
	return artifact, nil
}

func (cs *LuetCompiler) buildPackageImage(image, buildertaggedImage, packageImage string,
	concurrency int, keepPermissions bool,
	p *compilerspec.LuetCompilationSpec) (backend.Options, backend.Options, error) {

	var runnerOpts, builderOpts backend.Options

	pkgTag := ":package: " + p.GetPackage().HumanReadableString()

	// TODO:  Cleanup, not actually hit
	if packageImage == "" {
		return runnerOpts, builderOpts, errors.New("no package image given")
	}

	p.SetSeedImage(image) // In this case, we ignore the build deps as we suppose that the image has them - otherwise we recompose the tree with a solver,
	// and we build all the images first.

	err := os.MkdirAll(p.Rel("build"), os.ModePerm)
	if err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Error met while creating tempdir for building")
	}
	buildDir, err := ioutil.TempDir(p.Rel("build"), "pack")
	if err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Error met while creating tempdir for building")
	}
	defer os.RemoveAll(buildDir) // clean up

	// First we copy the source definitions into the output - we create a copy which the builds will need (we need to cache this phase somehow)
	err = fileHelper.CopyDir(p.GetPackage().GetPath(), buildDir)
	if err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not copy package sources")
	}

	// Copy file into the build context, the compilespec might have requested to do so.
	if len(p.GetRetrieve()) > 0 {
		err := p.CopyRetrieves(buildDir)
		if err != nil {
			Warning("Failed copying retrieves", err.Error())
		}
	}

	// First we create the builder image
	if err := p.WriteBuildImageDefinition(filepath.Join(buildDir, p.GetPackage().GetFingerPrint()+"-builder.dockerfile")); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not generate image definition")
	}

	// Even if we don't have prelude steps, we want to push
	// An intermediate image to tag images which are outside of the tree.
	// Those don't have an hash otherwise, and thus makes build unreproducible
	// see SKIPBUILD for the other logic
	// if len(p.GetPreBuildSteps()) == 0 {
	// 	buildertaggedImage = image
	// }
	// We might want to skip this phase but replacing with a tag that we push. But in case
	// steps in prelude are == 0 those are equivalent.

	// Then we write the step image, which uses the builder one
	if err := p.WriteStepImageDefinition(buildertaggedImage, filepath.Join(buildDir, p.GetPackage().GetFingerPrint()+".dockerfile")); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not generate image definition")
	}

	builderOpts = backend.Options{
		ImageName:      buildertaggedImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + "-builder.dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + "-builder.image.tar"),
		BackendArgs:    cs.Options.BackendArgs,
	}
	runnerOpts = backend.Options{
		ImageName:      packageImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + ".dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
		BackendArgs:    cs.Options.BackendArgs,
	}

	buildAndPush := func(opts backend.Options) error {
		buildImage := true
		if cs.Options.PullFirst {
			err := cs.Backend.DownloadImage(opts)
			if err == nil {
				buildImage = false
			} else {
				Warning("Failed to download '" + opts.ImageName + "'. Will keep going and build the image unless you use --fatal")
				Warning(err.Error())
			}
		}
		if buildImage {
			if err := cs.Backend.BuildImage(opts); err != nil {
				return errors.Wrapf(err, "Could not build image: %s %s", image, opts.DockerFileName)
			}
			if cs.Options.Push {
				if err = cs.Backend.Push(opts); err != nil {
					return errors.Wrapf(err, "Could not push image: %s %s", image, opts.DockerFileName)
				}
			}
		}
		return nil
	}
	// SKIPBUILD
	//	if len(p.GetPreBuildSteps()) != 0 {
	Info(pkgTag, ":whale: Generating 'builder' image from", image, "as", buildertaggedImage, "with prelude steps")
	if err := buildAndPush(builderOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, builderOpts.DockerFileName)
	}
	//}

	// Even if we might not have any steps to build, we do that so we can tag the image used in this moment and use that to cache it in a registry, or in the system.
	// acting as a docker tag.
	Info(pkgTag, ":whale: Generating 'package' image from", buildertaggedImage, "as", packageImage, "with build steps")
	if err := buildAndPush(runnerOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, runnerOpts.DockerFileName)
	}

	return builderOpts, runnerOpts, nil
}

func (cs *LuetCompiler) genArtifact(p *compilerspec.LuetCompilationSpec, builderOpts, runnerOpts backend.Options, concurrency int, keepPermissions bool) (*artifact.PackageArtifact, error) {

	// generate *artifact.PackageArtifact
	var a *artifact.PackageArtifact
	var rootfs string
	var err error
	pkgTag := ":package: " + p.GetPackage().HumanReadableString()
	Debug(pkgTag, "Generating artifact")
	// We can't generate delta in this case. It implies the package is a virtual, and nothing has to be done really
	if p.EmptyPackage() {
		fakePackage := p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar")

		rootfs, err = ioutil.TempDir(p.GetOutputPath(), "rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Could not create tempdir")
		}
		defer os.RemoveAll(rootfs) // clean up

		a := artifact.NewPackageArtifact(fakePackage)
		a.CompressionType = cs.Options.CompressionType

		if err := a.Compress(rootfs, concurrency); err != nil {
			return nil, errors.Wrap(err, "Error met while creating package archive")
		}

		a.CompileSpec = p
		a.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())

		err = a.WriteYaml(p.GetOutputPath())
		if err != nil {
			return a, errors.Wrap(err, "Failed while writing metadata file")
		}
		Info(pkgTag, "   :white_check_mark: done (empty virtual package)")
		return a, nil
	}

	if p.UnpackedPackage() {
		// Take content of container as a base for our package files
		a, err = cs.unpackFs(concurrency, keepPermissions, p, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while extracting image")
		}
	} else {
		// Generate delta between the two images
		a, err = cs.unpackDelta(concurrency, keepPermissions, p, builderOpts, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while generating delta")
		}
	}

	filelist, err := a.FileList()
	if err != nil {
		return a, errors.Wrapf(err, "Failed getting package list for '%s' '%s'", a.Path, a.CompileSpec.Package.HumanReadableString())
	}

	a.Files = filelist
	a.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())

	err = a.WriteYaml(p.GetOutputPath())
	if err != nil {
		return a, errors.Wrap(err, "Failed while writing metadata file")
	}
	Info(pkgTag, "   :white_check_mark: Done")

	return a, nil
}

func (cs *LuetCompiler) waitForImages(images []string) {
	if cs.Options.PullFirst && cs.Options.Wait {
		available, _ := oneOfImagesAvailable(images, cs.Backend)
		if !available {
			Info(fmt.Sprintf("Waiting for image %s to be available... :zzz:", images))
			Spinner(22)
			defer SpinnerStop()
			for !available {
				available, _ = oneOfImagesAvailable(images, cs.Backend)
				Info(fmt.Sprintf("Image %s not available yet, sleeping", images))
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func oneOfImagesExists(images []string, b CompilerBackend) (bool, string) {
	for _, i := range images {
		if exists := b.ImageExists(i); exists {
			return true, i
		}
	}
	return false, ""
}
func oneOfImagesAvailable(images []string, b CompilerBackend) (bool, string) {
	for _, i := range images {
		if exists := b.ImageAvailable(i); exists {
			return true, i
		}
	}
	return false, ""
}

func (cs *LuetCompiler) findImageHash(imageHash string, p *compilerspec.LuetCompilationSpec) string {
	var resolvedImage string
	Debug("Resolving image hash for", p.Package.HumanReadableString(), "hash", imageHash, "Pull repositories", p.BuildOptions.PullImageRepository)
	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, imageHash)},
		genImageList(p.BuildOptions.PullImageRepository, imageHash)...)
	if exists, which := oneOfImagesExists(toChecklist, cs.Backend); exists {
		resolvedImage = which
	}
	if cs.Options.PullFirst {
		if exists, which := oneOfImagesAvailable(toChecklist, cs.Backend); exists {
			resolvedImage = which
		}
	}
	return resolvedImage
}

func (cs *LuetCompiler) resolveExistingImageHash(imageHash string, p *compilerspec.LuetCompilationSpec) string {
	resolvedImage := cs.findImageHash(imageHash, p)

	if resolvedImage == "" {
		resolvedImage = fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, imageHash)
	}
	return resolvedImage
}

func LoadArtifactFromYaml(spec *compilerspec.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	metaFile := spec.GetPackage().GetMetadataFilePath()
	dat, err := ioutil.ReadFile(spec.Rel(metaFile))
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+metaFile)
	}
	art, err := artifact.NewPackageArtifactFromYaml(dat)
	if err != nil {
		return nil, errors.Wrap(err, "Error writing file "+metaFile)
	}
	// It is relative, set it back to abs
	art.Path = spec.Rel(art.Path)
	return art, nil
}

func (cs *LuetCompiler) getImageArtifact(hash string, p *compilerspec.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	// we check if there is an available image with the given hash and
	// we return a full artifact if can be loaded locally.
	Debug("Get image artifact for", p.Package.HumanReadableString(), "hash", hash, "Pull repositories", p.BuildOptions.PullImageRepository)

	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, hash)},
		genImageList(p.BuildOptions.PullImageRepository, hash)...)

	exists, _ := oneOfImagesExists(toChecklist, cs.Backend)
	if art, err := LoadArtifactFromYaml(p); err == nil && exists { // If YAML is correctly loaded, and both images exists, no reason to rebuild.
		Debug("Package reloaded from YAML. Skipping build")
		return art, nil
	}
	cs.waitForImages(toChecklist)
	available, _ := oneOfImagesAvailable(toChecklist, cs.Backend)
	if exists || (cs.Options.PullFirst && available) {
		Debug("Image available, returning empty artifact")
		return &artifact.PackageArtifact{}, nil
	}

	return nil, errors.New("artifact not found")
}

// compileWithImage compiles a PackageTagHash image using the image source, and tagging an indermediate
// image buildertaggedImage.
// Images that can be resolved from repositories are prefered over the local ones if PullFirst is set to true
// avoiding to rebuild images as much as possible
func (cs *LuetCompiler) compileWithImage(image, builderHash string, packageTagHash string,
	concurrency int,
	keepPermissions, keepImg bool,
	p *compilerspec.LuetCompilationSpec, generateArtifact bool) (*artifact.PackageArtifact, error) {

	// If it is a virtual, check if we have to generate an empty artifact or not.
	if generateArtifact && p.IsVirtual() {
		return cs.genArtifact(p, backend.Options{}, backend.Options{}, concurrency, keepPermissions)
	} else if p.IsVirtual() {
		return &artifact.PackageArtifact{}, nil
	}

	if !generateArtifact {
		if art, err := cs.getImageArtifact(packageTagHash, p); err == nil {
			// try to avoid regenerating the image if possible by checking the hash in the
			// given repositories
			// It is best effort. If we fail resolving, we will generate the images and keep going
			return art, nil
		}
	}

	packageImage := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, packageTagHash)
	remoteBuildertaggedImage := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, builderHash)
	builderResolved := cs.resolveExistingImageHash(builderHash, p)
	//generated := false
	// if buildertaggedImage == "" {
	// 	buildertaggedImage = fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, buildertaggedImage)
	// 	generated = true
	// 	//	Debug(pkgTag, "Creating intermediary image", buildertaggedImage, "from", image)
	// }

	if cs.Options.PullFirst && !cs.Options.Rebuild {
		Debug("Checking if an image is already available")
		// FIXUP here. If packageimage hash exists and pull is true, generate package
		resolved := cs.resolveExistingImageHash(packageTagHash, p)
		Debug("Resolved: " + resolved)
		Debug("Expected remote: " + resolved)
		Debug("Package image: " + packageImage)
		Debug("Resolved builder image: " + builderResolved)

		// a remote image is there already
		remoteImageAvailable := resolved != packageImage && remoteBuildertaggedImage != builderResolved
		// or a local one is available
		localImageAvailable := cs.Backend.ImageExists(remoteBuildertaggedImage) && cs.Backend.ImageExists(packageImage)

		switch {
		case remoteImageAvailable:
			Debug("Images available remotely for", p.Package.HumanReadableString(), "generating artifact from remote images:", resolved)
			return cs.genArtifact(p, backend.Options{ImageName: builderResolved}, backend.Options{ImageName: resolved}, concurrency, keepPermissions)
		case localImageAvailable:
			Debug("Images locally available for", p.Package.HumanReadableString(), "generating artifact from image:", resolved)
			return cs.genArtifact(p, backend.Options{ImageName: remoteBuildertaggedImage}, backend.Options{ImageName: packageImage}, concurrency, keepPermissions)
		default:
			Debug("Images not available for", p.Package.HumanReadableString())
		}
	}

	// always going to point at the destination from the repo defined
	builderOpts, runnerOpts, err := cs.buildPackageImage(image, builderResolved, packageImage, concurrency, keepPermissions, p)
	if err != nil {
		return nil, errors.Wrap(err, "failed building package image")
	}

	if !keepImg {
		defer func() {
			// We keep them around, so to not reload them from the tar (which should be the "correct way") and we automatically share the same layers
			if err := cs.Backend.RemoveImage(builderOpts); err != nil {
				Warning("Could not remove image ", builderOpts.ImageName)
			}
			if err := cs.Backend.RemoveImage(runnerOpts); err != nil {
				Warning("Could not remove image ", runnerOpts.ImageName)
			}
		}()
	}

	if !generateArtifact {
		return &artifact.PackageArtifact{}, nil
	}

	return cs.genArtifact(p, builderOpts, runnerOpts, concurrency, keepPermissions)
}

// FromDatabase returns all the available compilation specs from a database. If the minimum flag is returned
// it will be computed a minimal subset that will guarantees that all packages are built ( if not targeting a single package explictly )
func (cs *LuetCompiler) FromDatabase(db pkg.PackageDatabase, minimum bool, dst string) ([]*compilerspec.LuetCompilationSpec, error) {
	compilerSpecs := compilerspec.NewLuetCompilationspecs()

	w := db.World()

	for _, p := range w {
		spec, err := cs.FromPackage(p)
		if err != nil {
			return nil, err
		}
		if dst != "" {
			spec.SetOutputPath(dst)
		}
		compilerSpecs.Add(spec)
	}

	switch minimum {
	case true:
		return cs.ComputeMinimumCompilableSet(compilerSpecs.Unique().All()...)
	default:
		return compilerSpecs.Unique().All(), nil
	}
}

func (cs *LuetCompiler) ComputeDepTree(p *compilerspec.LuetCompilationSpec) (solver.PackagesAssertions, error) {
	s := solver.NewResolver(cs.Options.SolverOptions.Options, pkg.NewInMemoryDatabase(false), cs.Database, pkg.NewInMemoryDatabase(false), cs.Options.SolverOptions.Resolver())

	solution, err := s.Install(pkg.Packages{p.GetPackage()})
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().HumanReadableString())
	}

	dependencies, err := solution.Order(cs.Database, p.GetPackage().GetFingerPrint())
	if err != nil {
		return nil, errors.Wrap(err, "While order a solution for "+p.GetPackage().HumanReadableString())
	}
	return dependencies, nil
}

// ComputeMinimumCompilableSet strips specs that are eventually compiled by leafs
func (cs *LuetCompiler) ComputeMinimumCompilableSet(p ...*compilerspec.LuetCompilationSpec) ([]*compilerspec.LuetCompilationSpec, error) {
	// Generate a set with all the deps of the provided specs
	// we will use that set to remove the deps from the list of provided compilation specs
	allDependencies := solver.PackagesAssertions{} // Get all packages that will be in deps
	result := []*compilerspec.LuetCompilationSpec{}
	for _, spec := range p {
		sol, err := cs.ComputeDepTree(spec)
		if err != nil {
			return nil, errors.Wrap(err, "failed querying hashtree")
		}
		allDependencies = append(allDependencies, sol.Drop(spec.GetPackage())...)
	}

	for _, spec := range p {
		if found := allDependencies.Search(spec.GetPackage().GetFingerPrint()); found == nil {
			result = append(result, spec)
		}
	}
	return result, nil
}

// Compile is a non-parallel version of CompileParallel. It builds the compilation specs and generates
// an artifact
func (cs *LuetCompiler) Compile(keepPermissions bool, p *compilerspec.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	return cs.compile(cs.Options.Concurrency, keepPermissions, nil, nil, p)
}

func genImageList(refs []string, hash string) []string {
	var res []string
	for _, r := range refs {
		res = append(res, fmt.Sprintf("%s:%s", r, hash))
	}
	return res
}

func (cs *LuetCompiler) inheritSpecBuildOptions(p *compilerspec.LuetCompilationSpec) {
	Debug(p.GetPackage().HumanReadableString(), "Build options before inherit", p.BuildOptions)

	// Append push repositories from buildpsec buildoptions as pull if found.
	// This allows to resolve the hash automatically if we pulled the metadata from
	// repositories that are advertizing their cache.
	if len(p.BuildOptions.PushImageRepository) != 0 {
		p.BuildOptions.PullImageRepository = append(p.BuildOptions.PullImageRepository, p.BuildOptions.PushImageRepository)
		Debug("Inheriting pull repository from PushImageRepository buildoptions", p.BuildOptions.PullImageRepository)
	}

	if len(cs.Options.PullImageRepository) != 0 {
		p.BuildOptions.PullImageRepository = append(p.BuildOptions.PullImageRepository, cs.Options.PullImageRepository...)
		Debug("Inheriting pull repository from PullImageRepository buildoptions", p.BuildOptions.PullImageRepository)
	}

	Debug(p.GetPackage().HumanReadableString(), "Build options after inherit", p.BuildOptions)
}

func (cs *LuetCompiler) getSpecHash(pkgs pkg.DefaultPackages, salt string) (string, error) {
	ht := NewHashTree(cs.Database)
	overallFp := ""
	for _, p := range pkgs {
		compileSpec, err := cs.FromPackage(p)
		if err != nil {
			return "", errors.Wrap(err, "Error while generating compilespec for "+p.GetName())
		}
		packageHashTree, err := ht.Query(cs, compileSpec)
		if err != nil {
			return "nil", errors.Wrap(err, "failed querying hashtree")
		}
		overallFp = overallFp + packageHashTree.Target.Hash.PackageHash + p.GetFingerPrint()
	}

	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", overallFp, salt))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (cs *LuetCompiler) resolveFinalImages(concurrency int, keepPermissions bool, p *compilerspec.LuetCompilationSpec) error {

	joinTag := ">:loop: final images<"
	var fromPackages pkg.DefaultPackages

	if len(p.Join) > 0 {
		fromPackages = p.Join
		Warning(joinTag, `
	Attention! the 'join' keyword is going to be deprecated in Luet >=0.18.x. 
	Use 'requires_final_images: true' instead in the build.yaml file`)
	} else if p.RequiresFinalImages {
		Info(joinTag, "Generating a parent image from final packages")
		fromPackages = p.Package.GetRequires()
	} else {
		// No source image to resolve
		return nil
	}

	// First compute a hash and check if image is available. if it is, then directly consume that
	overallFp, err := cs.getSpecHash(fromPackages, "join")
	if err != nil {
		return errors.Wrap(err, "could not generate image hash")
	}

	Info(joinTag, "Searching existing image with hash", overallFp)

	image := cs.findImageHash(overallFp, p)
	if image != "" {
		Info("Image already found", image)
		p.SetImage(image)
		return nil
	}
	Info(joinTag, "Image not found. Generating image join with hash ", overallFp)

	// Make sure there is an output path
	if err := os.MkdirAll(p.GetOutputPath(), os.ModePerm); err != nil {
		return errors.Wrap(err, "while creating output path")
	}

	// otherwise, generate it and push it aside
	joinDir, err := ioutil.TempDir(p.GetOutputPath(), "join")
	if err != nil {
		return errors.Wrap(err, "could not create tempdir for joining images")
	}
	defer os.RemoveAll(joinDir) // clean up

	for _, p := range fromPackages {
		Info(joinTag, ":arrow_right_hook:", p.HumanReadableString(), ":leaves:")
	}

	current := 0
	for _, c := range fromPackages {
		current++
		if c != nil && c.Name != "" && c.Version != "" {
			joinTag2 := fmt.Sprintf("%s %d/%d ⤑ :hammer: build %s", joinTag, current, len(p.Join), c.HumanReadableString())

			Info(joinTag2, "compilation starts")
			spec, err := cs.FromPackage(c)
			if err != nil {
				return errors.Wrap(err, "while generating images to join from")
			}
			wantsArtifact := true
			genDepsArtifact := !cs.Options.PackageTargetOnly

			spec.SetOutputPath(p.GetOutputPath())

			artifact, err := cs.compile(concurrency, keepPermissions, &wantsArtifact, &genDepsArtifact, spec)
			if err != nil {
				return errors.Wrap(err, "failed building join image")
			}

			err = artifact.Unpack(joinDir, keepPermissions)
			if err != nil {
				return errors.Wrap(err, "failed building join image")
			}
			Info(joinTag2, ":white_check_mark: Done")
		}
	}

	artifactDir, err := ioutil.TempDir(p.GetOutputPath(), "artifact")
	if err != nil {
		return errors.Wrap(err, "could not create tempdir for final artifact")
	}
	defer os.RemoveAll(joinDir) // clean up

	Info(joinTag, ":droplet: generating artifact for source image of", p.GetPackage().HumanReadableString())

	// After unpack, create a new artifact and a new final image from it.
	// no need to compress, as we are going to toss it away.
	a := artifact.NewPackageArtifact(filepath.Join(artifactDir, p.GetPackage().GetFingerPrint()+".join.tar"))
	if err := a.Compress(joinDir, concurrency); err != nil {
		return errors.Wrap(err, "error met while creating package archive")
	}

	joinImageName := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, overallFp)
	Info(joinTag, ":droplet: generating image from artifact", joinImageName)
	opts, err := a.GenerateFinalImage(joinImageName, cs.Backend, keepPermissions)
	if err != nil {
		return errors.Wrap(err, "could not create final image")
	}
	if cs.Options.Push {
		Info(joinTag, ":droplet: pushing image from artifact", joinImageName)
		if err = cs.Backend.Push(opts); err != nil {
			return errors.Wrapf(err, "Could not push image: %s %s", image, opts.DockerFileName)
		}
	}
	Info(joinTag, ":droplet: Consuming image", joinImageName)
	p.SetImage(joinImageName)
	return nil
}

func (cs *LuetCompiler) resolveMultiStageImages(concurrency int, keepPermissions bool, p *compilerspec.LuetCompilationSpec) error {
	resolvedCopyFields := []compilerspec.CopyField{}
	copyTag := ">:droplet: copy<"

	if len(p.Copy) != 0 {
		Info(copyTag, "Package has multi-stage copy, generating required images")
	}

	current := 0
	// TODO: we should run this only if we are going to build the image
	for _, c := range p.Copy {
		current++
		if c.Package != nil && c.Package.Name != "" && c.Package.Version != "" {
			copyTag2 := fmt.Sprintf("%s %d/%d ⤑ :hammer: build %s", copyTag, current, len(p.Copy), c.Package.HumanReadableString())

			Info(copyTag2, "generating multi-stage images for", c.Package.HumanReadableString())
			spec, err := cs.FromPackage(c.Package)
			if err != nil {
				return errors.Wrap(err, "while generating images to copy from")
			}

			// If we specify --only-target package, we don't want any artifact, otherwise we do
			genArtifact := !cs.Options.PackageTargetOnly
			spec.SetOutputPath(p.GetOutputPath())
			artifact, err := cs.compile(concurrency, keepPermissions, &genArtifact, &genArtifact, spec)
			if err != nil {
				return errors.Wrap(err, "failed building multi-stage image")
			}

			resolvedCopyFields = append(resolvedCopyFields, compilerspec.CopyField{
				Image:       cs.resolveExistingImageHash(artifact.PackageCacheImage, spec),
				Source:      c.Source,
				Destination: c.Destination,
			})
			Info(copyTag2, ":white_check_mark: Done")
		} else {
			resolvedCopyFields = append(resolvedCopyFields, c)
		}
	}
	p.Copy = resolvedCopyFields
	return nil
}

func (cs *LuetCompiler) compile(concurrency int, keepPermissions bool, generateFinalArtifact *bool, generateDependenciesFinalArtifact *bool, p *compilerspec.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	Info(":package: Compiling", p.GetPackage().HumanReadableString(), ".... :coffee:")

	//Before multistage : join - same as multistage, but keep artifacts, join them, create a new one and generate a final image.
	// When the image is there, use it as a source here, in place of GetImage().
	if err := cs.resolveFinalImages(concurrency, keepPermissions, p); err != nil {
		return nil, errors.Wrap(err, "while resolving join images")
	}

	if err := cs.resolveMultiStageImages(concurrency, keepPermissions, p); err != nil {
		return nil, errors.Wrap(err, "while resolving multi-stage images")
	}

	Debug(fmt.Sprintf("%s: has images %t, empty package: %t", p.GetPackage().HumanReadableString(), p.HasImageSource(), p.EmptyPackage()))
	if !p.HasImageSource() && !p.EmptyPackage() {
		return nil,
			fmt.Errorf(
				"%s is invalid: package has no dependencies and no seed image supplied while it has steps defined",
				p.GetPackage().GetFingerPrint(),
			)
	}

	ht := NewHashTree(cs.Database)

	packageHashTree, err := ht.Query(cs, p)
	if err != nil {
		return nil, errors.Wrap(err, "failed querying hashtree")
	}

	// This is in order to have the metadata in the yaml
	p.SetSourceAssertion(packageHashTree.Solution)
	targetAssertion := packageHashTree.Target

	bus.Manager.Publish(bus.EventPackagePreBuild, struct {
		CompileSpec     *compilerspec.LuetCompilationSpec
		Assert          solver.PackageAssert
		PackageHashTree *PackageImageHashTree
	}{
		CompileSpec:     p,
		Assert:          *targetAssertion,
		PackageHashTree: packageHashTree,
	})

	// Update compilespec build options - it will be then serialized into the compilation metadata file
	p.BuildOptions.PushImageRepository = cs.Options.PushImageRepository

	// - If image is set we just generate a plain dockerfile
	// Treat last case (easier) first. The image is provided and we just compute a plain dockerfile with the images listed as above
	if p.GetImage() != "" {
		localGenerateArtifact := true
		if generateFinalArtifact != nil {
			localGenerateArtifact = *generateFinalArtifact
		}

		a, err := cs.compileWithImage(p.GetImage(), packageHashTree.BuilderImageHash, targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.Options.KeepImg, p, localGenerateArtifact)
		if err != nil {
			return nil, errors.Wrap(err, "building direct image")
		}
		a.SourceAssertion = p.GetSourceAssertion()

		a.PackageCacheImage = targetAssertion.Hash.PackageHash
		return a, nil
	}

	// - If image is not set, we read a base_image. Then we will build one image from it to kick-off our build based
	// on how we compute the resolvable tree.
	// This means to recursively build all the build-images needed to reach that tree part.
	// - We later on compute an hash used to identify the image, so each similar deptree keeps the same build image.
	dependencies := packageHashTree.Dependencies  // at this point we should have a flattened list of deps to build, including all of them (with all constraints propagated already)
	departifacts := []*artifact.PackageArtifact{} // TODO: Return this somehow
	depsN := 0
	currentN := 0

	packageDeps := !cs.Options.PackageTargetOnly
	if generateDependenciesFinalArtifact != nil {
		packageDeps = *generateDependenciesFinalArtifact
	}

	buildDeps := !cs.Options.NoDeps
	buildTarget := !cs.Options.OnlyDeps

	if buildDeps {
		Info(":deciduous_tree: Build dependencies for " + p.GetPackage().HumanReadableString())
		for _, assertion := range dependencies { //highly dependent on the order
			depsN++
			Info(" :arrow_right_hook:", assertion.Package.HumanReadableString(), ":leaves:")
		}

		for _, assertion := range dependencies { //highly dependent on the order
			currentN++
			pkgTag := fmt.Sprintf(":package: %d/%d %s ⤑ :hammer: build %s", currentN, depsN, p.GetPackage().HumanReadableString(), assertion.Package.HumanReadableString())
			Info(pkgTag, " starts")
			compileSpec, err := cs.FromPackage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "Error while generating compilespec for "+assertion.Package.GetName())
			}
			compileSpec.BuildOptions.PullImageRepository = append(compileSpec.BuildOptions.PullImageRepository, p.BuildOptions.PullImageRepository...)
			Debug("PullImage repos:", compileSpec.BuildOptions.PullImageRepository)

			compileSpec.SetOutputPath(p.GetOutputPath())

			bus.Manager.Publish(bus.EventPackagePreBuild, struct {
				CompileSpec *compilerspec.LuetCompilationSpec
				Assert      solver.PackageAssert
			}{
				CompileSpec: compileSpec,
				Assert:      assertion,
			})

			if err := cs.resolveFinalImages(concurrency, keepPermissions, compileSpec); err != nil {
				return nil, errors.Wrap(err, "while resolving join images")
			}

			if err := cs.resolveMultiStageImages(concurrency, keepPermissions, compileSpec); err != nil {
				return nil, errors.Wrap(err, "while resolving multi-stage images")
			}

			buildHash, err := packageHashTree.DependencyBuildImage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "failed looking for dependency in hashtree")
			}

			Debug(pkgTag, "    :arrow_right_hook: :whale: Builder image from hash", assertion.Hash.BuildHash)
			Debug(pkgTag, "    :arrow_right_hook: :whale: Package image from hash", assertion.Hash.PackageHash)

			var sourceImage string

			if compileSpec.GetImage() != "" {
				Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from image")
				sourceImage = compileSpec.GetImage()
			} else {
				// for the source instead, pick an image and a buildertaggedImage from hashes if they exists.
				// otherways fallback to the pushed repo
				// Resolve images from the hashtree
				sourceImage = cs.resolveExistingImageHash(assertion.Hash.BuildHash, compileSpec)
				Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from tree")
			}

			a, err := cs.compileWithImage(
				sourceImage,
				buildHash,
				assertion.Hash.PackageHash,
				concurrency,
				keepPermissions,
				cs.Options.KeepImg,
				compileSpec,
				packageDeps,
			)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
			}

			a.PackageCacheImage = assertion.Hash.PackageHash

			Info(pkgTag, ":white_check_mark: Done")

			bus.Manager.Publish(bus.EventPackagePostBuild, struct {
				CompileSpec *compilerspec.LuetCompilationSpec
				Artifact    *artifact.PackageArtifact
			}{
				CompileSpec: compileSpec,
				Artifact:    a,
			})

			departifacts = append(departifacts, a)
		}
	}

	if buildTarget {
		localGenerateArtifact := true
		if generateFinalArtifact != nil {
			localGenerateArtifact = *generateFinalArtifact
		}
		resolvedSourceImage := cs.resolveExistingImageHash(packageHashTree.SourceHash, p)
		Info(":rocket: All dependencies are satisfied, building package requested by the user", p.GetPackage().HumanReadableString())
		Info(":package:", p.GetPackage().HumanReadableString(), " Using image: ", resolvedSourceImage)
		a, err := cs.compileWithImage(resolvedSourceImage, packageHashTree.BuilderImageHash, targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.Options.KeepImg, p, localGenerateArtifact)
		if err != nil {
			return a, err
		}
		a.Dependencies = departifacts
		a.SourceAssertion = p.GetSourceAssertion()
		a.PackageCacheImage = targetAssertion.Hash.PackageHash
		bus.Manager.Publish(bus.EventPackagePostBuild, struct {
			CompileSpec *compilerspec.LuetCompilationSpec
			Artifact    *artifact.PackageArtifact
		}{
			CompileSpec: p,
			Artifact:    a,
		})

		return a, err
	} else {
		return departifacts[len(departifacts)-1], nil
	}
}

type templatedata map[string]interface{}

func (cs *LuetCompiler) templatePackage(vals []map[string]interface{}, pack pkg.Package, dst templatedata) ([]byte, error) {
	// Grab shared templates first
	var chartFiles []*chart.File
	if len(cs.Options.TemplatesFolder) != 0 {
		c, err := helpers.ChartFiles(cs.Options.TemplatesFolder)
		if err == nil {
			chartFiles = c
		}
	}

	var dataresult []byte
	val := pack.Rel(DefinitionFile)

	if _, err := os.Stat(pack.Rel(CollectionFile)); err == nil {
		val = pack.Rel(CollectionFile)

		data, err := ioutil.ReadFile(val)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+val)
		}

		dataBuild, err := ioutil.ReadFile(pack.Rel(BuildFile))
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+val)
		}

		packsRaw, err := pkg.GetRawPackages(data)
		if err != nil {
			return nil, errors.Wrap(err, "getting raw packages")
		}

		raw := packsRaw.Find(pack.GetName(), pack.GetCategory(), pack.GetVersion())
		td := templatedata{}
		if len(vals) > 0 {
			for _, bv := range vals {
				current := templatedata(bv)
				if err := mergo.Merge(&td, current); err != nil {
					return nil, errors.Wrap(err, "merging values maps")
				}
			}
		}

		if err := mergo.Merge(&td, templatedata(raw)); err != nil {
			return nil, errors.Wrap(err, "merging values maps")
		}

		dat, err := helpers.RenderHelm(append(chartFiles, helpers.ChartFileB(dataBuild)...), td, dst)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(dat)
	} else {
		bv := cs.Options.BuildValuesFile
		if len(vals) > 0 {
			valuesdir, err := ioutil.TempDir("", "genvalues")
			if err != nil {
				return nil, errors.Wrap(err, "Could not create tempdir")
			}
			defer os.RemoveAll(valuesdir) // clean up
			for _, b := range vals {
				out, err := yaml.Marshal(b)
				if err != nil {
					return nil, errors.Wrap(err, "while marshalling values file")
				}
				f := filepath.Join(valuesdir, fileHelper.RandStringRunes(20))
				if err := ioutil.WriteFile(f, out, os.ModePerm); err != nil {
					return nil, errors.Wrap(err, "while writing temporary values file")
				}
				bv = append([]string{f}, bv...)
			}
		}

		raw, err := ioutil.ReadFile(pack.Rel(BuildFile))
		if err != nil {
			return nil, err
		}

		out, err := helpers.RenderFiles(append(chartFiles, helpers.ChartFileB(raw)...), val, bv...)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(out)
	}

	return dataresult, nil
}

// FromPackage returns a compilation spec from a package definition
func (cs *LuetCompiler) FromPackage(p pkg.Package) (*compilerspec.LuetCompilationSpec, error) {

	pack, err := cs.Database.FindPackageCandidate(p)
	if err != nil {
		return nil, err
	}

	opts := options.Compiler{}

	artifactMetadataFile := filepath.Join(pack.GetTreeDir(), "..", pack.GetMetadataFilePath())
	Debug("Checking if metadata file is present", artifactMetadataFile)
	if _, err := os.Stat(artifactMetadataFile); err == nil {
		f, err := os.Open(artifactMetadataFile)
		if err != nil {
			return nil, errors.Wrapf(err, "could not open %s", artifactMetadataFile)
		}
		dat, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		art, err := artifact.NewPackageArtifactFromYaml(dat)
		if err != nil {
			return nil, errors.Wrap(err, "could not decode package from yaml")
		}

		Debug("Read build options:", art.CompileSpec.BuildOptions, "from", artifactMetadataFile)
		if art.CompileSpec.BuildOptions != nil {
			opts = *art.CompileSpec.BuildOptions
		}
	} else if !os.IsNotExist(err) {
		Debug("error reading artifact metadata file: ", err.Error())
	} else if os.IsNotExist(err) {
		Debug("metadata file not present, skipping", artifactMetadataFile)
	}

	// Update processed build values
	dst, err := helpers.UnMarshalValues(cs.Options.BuildValuesFile)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling values")
	}
	opts.BuildValues = append(opts.BuildValues, (map[string]interface{})(dst))

	bytes, err := cs.templatePackage(opts.BuildValues, pack, templatedata(dst))
	if err != nil {
		return nil, errors.Wrap(err, "while rendering package template")
	}

	newSpec, err := compilerspec.NewLuetCompilationSpec(bytes, pack)
	if err != nil {
		return nil, err
	}
	newSpec.BuildOptions = &opts

	cs.inheritSpecBuildOptions(newSpec)

	// Update the package in the compiler database to catch updates from NewLuetCompilationSpec
	if err := cs.Database.UpdatePackage(newSpec.Package); err != nil {
		return nil, errors.Wrap(err, "failed updating new package entry in compiler database")
	}

	return newSpec, err
}

// GetBackend returns the current compilation backend
func (cs *LuetCompiler) GetBackend() CompilerBackend {
	return cs.Backend
}

// SetBackend sets the compilation backend
func (cs *LuetCompiler) SetBackend(b CompilerBackend) {
	cs.Backend = b
}
