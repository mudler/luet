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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"regexp"
	"strings"
	"sync"
	"time"

	bus "github.com/mudler/luet/pkg/bus"
	yaml "gopkg.in/yaml.v2"

	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	"github.com/pkg/errors"
)

const BuildFile = "build.yaml"
const DefinitionFile = "definition.yaml"
const CollectionFile = "collection.yaml"

type LuetCompiler struct {
	*tree.CompilerRecipe
	Backend             CompilerBackend
	Database            pkg.PackageDatabase
	PushImageRepository string
	PullImageRepository []string

	PullFirst, KeepImg, Clean bool
	Concurrency               int
	CompressionType           CompressionImplementation
	Options                   CompilerOptions
	SolverOptions             solver.Options
	BackedArgs                []string
}

func NewLuetCompiler(backend CompilerBackend, db pkg.PackageDatabase, opt *CompilerOptions, solvopts solver.Options) Compiler {
	// The CompilerRecipe will gives us a tree with only build deps listed.

	if len(opt.PullImageRepository) == 0 {
		opt.PullImageRepository = []string{opt.PushImageRepository}
	}

	return &LuetCompiler{
		Backend: backend,
		CompilerRecipe: &tree.CompilerRecipe{
			Recipe: tree.Recipe{Database: db},
		},
		Database:            db,
		PushImageRepository: opt.PushImageRepository,
		PullImageRepository: opt.PullImageRepository,
		PullFirst:           opt.PullFirst,
		CompressionType:     opt.CompressionType,
		KeepImg:             opt.KeepImg,
		Concurrency:         opt.Concurrency,
		Options:             *opt,
		SolverOptions:       solvopts,
	}
}

// SetBackendArgs sets arbitrary backend arguments.
// Those for example can be commands passed to the docker daemon during build phase,
// as build-args, etc.
func (cs *LuetCompiler) SetBackendArgs(args []string) {
	cs.BackedArgs = args
}

// SetConcurrency sets the compiler concurrency
func (cs *LuetCompiler) SetConcurrency(i int) {
	cs.Concurrency = i
}

// SetCompressionType sets the compiler compression type for resulting artifacts
func (cs *LuetCompiler) SetCompressionType(t CompressionImplementation) {
	cs.CompressionType = t
}

func (cs *LuetCompiler) compilerWorker(i int, wg *sync.WaitGroup, cspecs chan CompilationSpec, a *[]Artifact, m *sync.Mutex, concurrency int, keepPermissions bool, errors chan error) {
	defer wg.Done()

	for s := range cspecs {
		ar, err := cs.compile(concurrency, keepPermissions, s)
		if err != nil {
			errors <- err
		}

		m.Lock()
		*a = append(*a, ar)
		m.Unlock()
	}
}

// CompileWithReverseDeps compiles the supplied compilationspecs and their reverse dependencies
func (cs *LuetCompiler) CompileWithReverseDeps(keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error) {
	artifacts, err := cs.CompileParallel(keepPermissions, ps)
	if len(err) != 0 {
		return artifacts, err
	}

	Info(":ant: Resolving reverse dependencies")
	toCompile := NewLuetCompilationspecs()
	for _, a := range artifacts {

		revdeps := a.GetCompileSpec().GetPackage().Revdeps(cs.Database)
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
func (cs *LuetCompiler) CompileParallel(keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error) {
	all := make(chan CompilationSpec)
	artifacts := []Artifact{}
	mutex := &sync.Mutex{}
	errors := make(chan error, ps.Len())
	var wg = new(sync.WaitGroup)
	for i := 0; i < cs.Concurrency; i++ {
		wg.Add(1)
		go cs.compilerWorker(i, wg, all, &artifacts, mutex, cs.Concurrency, keepPermissions, errors)
	}

	for _, p := range ps.All() {
		asserts, err := cs.ComputeDepTree(p)
		if err != nil {
			panic(err)
		}
		p.SetSourceAssertion(asserts)
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
			}
		}

		if include && !match || !include && match {
			toRemove = append(toRemove, currentpath)
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

func (cs *LuetCompiler) unpackFs(concurrency int, keepPermissions bool, p CompilationSpec, runnerOpts CompilerBackendOptions) (Artifact, error) {

	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	err = cs.Backend.ExtractRootfs(CompilerBackendOptions{
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
	artifact := NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar"))
	artifact.SetCompressionType(cs.CompressionType)

	if err := artifact.Compress(rootfs, concurrency); err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}

	artifact.SetCompileSpec(p)
	return artifact, nil
}

func (cs *LuetCompiler) unpackDelta(concurrency int, keepPermissions bool, p CompilationSpec, builderOpts, runnerOpts CompilerBackendOptions) (Artifact, error) {

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
	diffs, err := cs.Backend.Changes(builderOpts, runnerOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate changes from layers")
	}

	Debug("Extracting image to grab files from delta")
	if err := cs.Backend.ExtractRootfs(CompilerBackendOptions{
		ImageName: runnerOpts.ImageName, Destination: rootfs}, keepPermissions); err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}
	artifact, err := ExtractArtifactFromDelta(rootfs, p.Rel(p.GetPackage().GetFingerPrint()+".package.tar"), diffs, concurrency, keepPermissions, p.GetIncludes(), p.GetExcludes(), cs.CompressionType)
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate deltas")
	}

	artifact.SetCompileSpec(p)
	return artifact, nil
}

func (cs *LuetCompiler) buildPackageImage(image, buildertaggedImage, packageImage string,
	concurrency int, keepPermissions bool,
	p CompilationSpec) (CompilerBackendOptions, CompilerBackendOptions, error) {

	var runnerOpts, builderOpts CompilerBackendOptions

	pkgTag := ":package: " + p.GetPackage().HumanReadableString()

	// Use packageImage as salt into the fp being used
	// so the hash is unique also in cases where
	// some package deps does have completely different
	// depgraphs
	// TODO: We should use the image tag, or pass by the package assertion hash which is unique
	// and identifies the deptree of the package.

	fp := p.GetPackage().HashFingerprint(helpers.StripRegistryFromImage(packageImage))

	if buildertaggedImage == "" {
		buildertaggedImage = cs.PushImageRepository + ":builder-" + fp
		Debug(pkgTag, "Creating intermediary image", buildertaggedImage, "from", image)
	}

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
	err = helpers.CopyDir(p.GetPackage().GetPath(), buildDir)
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

	if len(p.GetPreBuildSteps()) == 0 {
		buildertaggedImage = image
	}

	// Then we write the step image, which uses the builder one
	if err := p.WriteStepImageDefinition(buildertaggedImage, filepath.Join(buildDir, p.GetPackage().GetFingerPrint()+".dockerfile")); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not generate image definition")
	}

	builderOpts = CompilerBackendOptions{
		ImageName:      buildertaggedImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + "-builder.dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + "-builder.image.tar"),
		BackendArgs:    cs.BackedArgs,
	}
	runnerOpts = CompilerBackendOptions{
		ImageName:      packageImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + ".dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
		BackendArgs:    cs.BackedArgs,
	}

	buildAndPush := func(opts CompilerBackendOptions) error {
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
	if len(p.GetPreBuildSteps()) != 0 {
		Info(pkgTag, ":whale: Generating 'builder' image from", image, "as", buildertaggedImage, "with prelude steps")
		if err := buildAndPush(builderOpts); err != nil {
			return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, builderOpts.DockerFileName)
		}
	}

	// Even if we might not have any steps to build, we do that so we can tag the image used in this moment and use that to cache it in a registry, or in the system.
	// acting as a docker tag.
	Info(pkgTag, ":whale: Generating 'package' image from", buildertaggedImage, "as", packageImage, "with build steps")
	if err := buildAndPush(runnerOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, runnerOpts.DockerFileName)
	}

	return builderOpts, runnerOpts, nil
}

func (cs *LuetCompiler) genArtifact(p CompilationSpec, builderOpts, runnerOpts CompilerBackendOptions, concurrency int, keepPermissions bool) (Artifact, error) {

	// generate Artifact
	var artifact Artifact
	var rootfs string
	var err error
	pkgTag := ":package: " + p.GetPackage().HumanReadableString()

	// We can't generate delta in this case. It implies the package is a virtual, and nothing has to be done really
	if p.EmptyPackage() {
		fakePackage := p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar")

		rootfs, err = ioutil.TempDir(p.GetOutputPath(), "rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Could not create tempdir")
		}
		defer os.RemoveAll(rootfs) // clean up

		artifact := NewPackageArtifact(fakePackage)
		artifact.SetCompressionType(cs.CompressionType)

		if err := artifact.Compress(rootfs, concurrency); err != nil {
			return nil, errors.Wrap(err, "Error met while creating package archive")
		}

		artifact.SetCompileSpec(p)
		artifact.GetCompileSpec().GetPackage().SetBuildTimestamp(time.Now().String())

		err = artifact.WriteYaml(p.GetOutputPath())
		if err != nil {
			return artifact, errors.Wrap(err, "Failed while writing metadata file")
		}
		Info(pkgTag, "   :white_check_mark: done (empty virtual package)")
		return artifact, nil
	}

	if p.UnpackedPackage() {
		// Take content of container as a base for our package files
		artifact, err = cs.unpackFs(concurrency, keepPermissions, p, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while extracting image")
		}
	} else {
		// Generate delta between the two images
		artifact, err = cs.unpackDelta(concurrency, keepPermissions, p, builderOpts, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while generating delta")
		}
	}

	filelist, err := artifact.FileList()
	if err != nil {
		return artifact, errors.Wrap(err, "Failed getting package list")
	}

	artifact.SetFiles(filelist)
	artifact.GetCompileSpec().GetPackage().SetBuildTimestamp(time.Now().String())

	err = artifact.WriteYaml(p.GetOutputPath())
	if err != nil {
		return artifact, errors.Wrap(err, "Failed while writing metadata file")
	}
	Info(pkgTag, "   :white_check_mark: Done")

	return artifact, nil
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

func (cs *LuetCompiler) resolveExistingImageHash(imageHash string) string {
	var resolvedImage string
	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.PushImageRepository, imageHash)},
		genImageList(cs.PullImageRepository, imageHash)...)
	if exists, which := oneOfImagesExists(toChecklist, cs.Backend); exists {
		resolvedImage = which
	}
	if cs.Options.PullFirst {
		if exists, which := oneOfImagesAvailable(toChecklist, cs.Backend); exists {
			resolvedImage = which
		}
	}

	if resolvedImage == "" {
		resolvedImage = fmt.Sprintf("%s:%s", cs.PushImageRepository, imageHash)
	}
	return resolvedImage
}

func (cs *LuetCompiler) getImageArtifact(hash string, p CompilationSpec) (Artifact, error) {
	// we check if there is an available image with the given hash and
	// we return a full artifact if can be loaded locally.

	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.PushImageRepository, hash)},
		genImageList(cs.PullImageRepository, hash)...)

	exists, _ := oneOfImagesExists(toChecklist, cs.Backend)
	if art, err := LoadArtifactFromYaml(p); err == nil && exists { // If YAML is correctly loaded, and both images exists, no reason to rebuild.
		Debug("Artifact reloaded from YAML. Skipping build")
		return art, nil
	}
	cs.waitForImages(toChecklist)
	available, _ := oneOfImagesAvailable(toChecklist, cs.Backend)
	if exists || (cs.Options.PullFirst && available) {
		Debug("Image available, returning empty artifact")
		return &PackageArtifact{}, nil
	}

	return nil, errors.New("artifact not found")
}

// compileWithImage compiles a PackageTagHash image using the image source, and tagging an indermediate
// image buildertaggedImage.
// Images that can be resolved from repositories are prefered over the local ones if PullFirst is set to true
// avoiding to rebuild images as much as possible
func (cs *LuetCompiler) compileWithImage(image, buildertaggedImage string, packageTagHash string,
	concurrency int,
	keepPermissions, keepImg bool,
	p CompilationSpec, generateArtifact bool) (Artifact, error) {

	// If it is a virtual, check if we have to generate an empty artifact or not.
	if generateArtifact && p.IsVirtual() {
		return cs.genArtifact(p, CompilerBackendOptions{}, CompilerBackendOptions{}, concurrency, keepPermissions)
	} else if p.IsVirtual() {
		return &PackageArtifact{}, nil
	}

	if !generateArtifact {
		// try to avoid regenerating the image if possible by checking the hash in the
		// given repositories
		// It is best effort. If we fail resolving, we will generate the images and keep going
		if art, err := cs.getImageArtifact(packageTagHash, p); err == nil {
			return art, nil
		}
	}

	// always going to point at the destination from the repo defined
	packageImage := fmt.Sprintf("%s:%s", cs.PushImageRepository, packageTagHash)
	builderOpts, runnerOpts, err := cs.buildPackageImage(image, buildertaggedImage, packageImage, concurrency, keepPermissions, p)
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
		return &PackageArtifact{}, nil
	}

	return cs.genArtifact(p, builderOpts, runnerOpts, concurrency, keepPermissions)
}

// FromDatabase returns all the available compilation specs from a database. If the minimum flag is returned
// it will be computed a minimal subset that will guarantees that all packages are built ( if not targeting a single package explictly )
func (cs *LuetCompiler) FromDatabase(db pkg.PackageDatabase, minimum bool, dst string) ([]CompilationSpec, error) {
	compilerSpecs := NewLuetCompilationspecs()

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

// ComputeMinimumCompilableSet strips specs that are eventually compiled by leafs
func (cs *LuetCompiler) ComputeMinimumCompilableSet(p ...CompilationSpec) ([]CompilationSpec, error) {
	// Generate a set with all the deps of the provided specs
	// we will use that set to remove the deps from the list of provided compilation specs
	allDependencies := solver.PackagesAssertions{} // Get all packages that will be in deps
	result := []CompilationSpec{}
	for _, spec := range p {
		ass, err := cs.ComputeDepTree(spec)
		if err != nil {
			return result, errors.Wrap(err, "computin specs deptree")
		}

		allDependencies = append(allDependencies, ass.Drop(spec.GetPackage())...)
	}

	for _, spec := range p {
		if found := allDependencies.Search(spec.GetPackage().GetFingerPrint()); found == nil {
			result = append(result, spec)
		}
	}
	return result, nil
}

// ComputeDepTree computes the dependency tree of a compilation spec and returns solver assertions
// in order to be able to compile the spec.
func (cs *LuetCompiler) ComputeDepTree(p CompilationSpec) (solver.PackagesAssertions, error) {

	s := solver.NewResolver(cs.SolverOptions, pkg.NewInMemoryDatabase(false), cs.Database, pkg.NewInMemoryDatabase(false), cs.Options.SolverOptions.Resolver())

	solution, err := s.Install(pkg.Packages{p.GetPackage()})
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().HumanReadableString())
	}

	dependencies, err := solution.Order(cs.Database, p.GetPackage().GetFingerPrint())
	if err != nil {
		return nil, errors.Wrap(err, "While order a solution for "+p.GetPackage().HumanReadableString())
	}

	assertions := solver.PackagesAssertions{}
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value {
			nthsolution := dependencies.Cut(assertion.Package)
			assertion.Hash = solver.PackageHash{
				BuildHash:   nthsolution.HashFrom(assertion.Package),
				PackageHash: nthsolution.AssertionHash(),
			}
			assertions = append(assertions, assertion)
		}
	}
	p.SetSourceAssertion(assertions)
	return assertions, nil
}

// Compile is a non-parallel version of CompileParallel. It builds the compilation specs and generates
// an artifact
func (cs *LuetCompiler) Compile(keepPermissions bool, p CompilationSpec) (Artifact, error) {
	asserts, err := cs.ComputeDepTree(p)
	if err != nil {
		panic(err)
	}
	p.SetSourceAssertion(asserts)
	return cs.compile(cs.Concurrency, keepPermissions, p)
}

func genImageList(refs []string, hash string) []string {
	var res []string
	for _, r := range refs {
		res = append(res, fmt.Sprintf("%s:%s", r, hash))
	}
	return res
}

func (cs *LuetCompiler) compile(concurrency int, keepPermissions bool, p CompilationSpec) (Artifact, error) {
	Info(":package: Compiling", p.GetPackage().HumanReadableString(), ".... :coffee:")

	Debug(fmt.Sprintf("%s: has images %t, empty package: %t", p.GetPackage().HumanReadableString(), p.HasImageSource(), p.EmptyPackage()))
	if !p.HasImageSource() && !p.EmptyPackage() {
		return nil,
			fmt.Errorf(
				"%s is invalid: package has no dependencies and no seed image supplied while it has steps defined",
				p.GetPackage().GetFingerPrint(),
			)
	}

	targetAssertion := p.GetSourceAssertion().Search(p.GetPackage().GetFingerPrint())

	bus.Manager.Publish(bus.EventPackagePreBuild, struct {
		CompileSpec CompilationSpec
		Assert      solver.PackageAssert
	}{
		CompileSpec: p,
		Assert:      *targetAssertion,
	})

	// - If image is set we just generate a plain dockerfile
	// Treat last case (easier) first. The image is provided and we just compute a plain dockerfile with the images listed as above
	if p.GetImage() != "" {
		return cs.compileWithImage(p.GetImage(), "", targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.KeepImg, p, true)
	}

	// - If image is not set, we read a base_image. Then we will build one image from it to kick-off our build based
	// on how we compute the resolvable tree.
	// This means to recursively build all the build-images needed to reach that tree part.
	// - We later on compute an hash used to identify the image, so each similar deptree keeps the same build image.

	dependencies := p.GetSourceAssertion().Drop(p.GetPackage()) // at this point we should have a flattened list of deps to build, including all of them (with all constraints propagated already)
	departifacts := []Artifact{}                                // TODO: Return this somehow
	var lastHash string
	depsN := 0
	currentN := 0

	packageDeps := !cs.Options.PackageTargetOnly
	if !cs.Options.NoDeps {
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
			compileSpec.SetOutputPath(p.GetOutputPath())
			Debug(pkgTag, "    :arrow_right_hook: :whale: Builder image from hash", assertion.Hash.BuildHash)
			Debug(pkgTag, "    :arrow_right_hook: :whale: Package image from hash", assertion.Hash.PackageHash)

			bus.Manager.Publish(bus.EventPackagePreBuild, struct {
				CompileSpec CompilationSpec
				Assert      solver.PackageAssert
			}{
				CompileSpec: compileSpec,
				Assert:      assertion,
			})

			lastHash = assertion.Hash.PackageHash
			// for the source instead, pick an image and a buildertaggedImage from hashes if they exists.
			// otherways fallback to the pushed repo
			// Resolve images from the hashtree
			resolvedBuildImage := cs.resolveExistingImageHash(assertion.Hash.BuildHash)
			if compileSpec.GetImage() != "" {
				Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from image")

				artifact, err := cs.compileWithImage(compileSpec.GetImage(), resolvedBuildImage, assertion.Hash.PackageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec, packageDeps)
				if err != nil {
					return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
				}
				departifacts = append(departifacts, artifact)
				Info(pkgTag, ":white_check_mark: Done")
				continue
			}

			Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from tree")
			artifact, err := cs.compileWithImage(resolvedBuildImage, "", assertion.Hash.PackageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec, packageDeps)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
			}

			bus.Manager.Publish(bus.EventPackagePostBuild, struct {
				CompileSpec CompilationSpec
				Artifact    Artifact
			}{
				CompileSpec: compileSpec,
				Artifact:    artifact,
			})

			departifacts = append(departifacts, artifact)
			Info(pkgTag, ":white_check_mark: Done")
		}

	} else if len(dependencies) > 0 {
		lastHash = dependencies[len(dependencies)-1].Hash.PackageHash
	}

	if !cs.Options.OnlyDeps {
		resolvedBuildImage := cs.resolveExistingImageHash(lastHash)
		Info(":rocket: All dependencies are satisfied, building package requested by the user", p.GetPackage().HumanReadableString())
		Info(":package:", p.GetPackage().HumanReadableString(), " Using image: ", resolvedBuildImage)
		artifact, err := cs.compileWithImage(resolvedBuildImage, "", targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.KeepImg, p, true)
		if err != nil {
			return artifact, err
		}
		artifact.SetDependencies(departifacts)
		artifact.SetSourceAssertion(p.GetSourceAssertion())

		bus.Manager.Publish(bus.EventPackagePostBuild, struct {
			CompileSpec CompilationSpec
			Artifact    Artifact
		}{
			CompileSpec: p,
			Artifact:    artifact,
		})

		return artifact, err
	} else {
		return departifacts[len(departifacts)-1], nil
	}
}

type templatedata map[string]interface{}

func (cs *LuetCompiler) templatePackage(pack pkg.Package) ([]byte, error) {

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

		dst, err := helpers.UnMarshalValues(cs.Options.BuildValuesFile)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling values")
		}

		dat, err := helpers.RenderHelm(string(dataBuild), raw, dst)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(dat)
	} else {
		out, err := helpers.RenderFiles(pack.Rel(BuildFile), val, cs.Options.BuildValuesFile...)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(out)
	}
	return dataresult, nil

}

// FromPackage returns a compilation spec from a package definition
func (cs *LuetCompiler) FromPackage(p pkg.Package) (CompilationSpec, error) {

	pack, err := cs.Database.FindPackageCandidate(p)
	if err != nil {
		return nil, err
	}

	bytes, err := cs.templatePackage(pack)
	if err != nil {
		return nil, errors.Wrap(err, "while rendering package template")
	}

	return NewLuetCompilationSpec(bytes, pack)
}

// GetBackend returns the current compilation backend
func (cs *LuetCompiler) GetBackend() CompilerBackend {
	return cs.Backend
}

// SetBackend sets the compilation backend
func (cs *LuetCompiler) SetBackend(b CompilerBackend) {
	cs.Backend = b
}
