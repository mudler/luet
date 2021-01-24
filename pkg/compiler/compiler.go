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
	Backend                   CompilerBackend
	Database                  pkg.PackageDatabase
	ImageRepository           string
	PullFirst, KeepImg, Clean bool
	Concurrency               int
	CompressionType           CompressionImplementation
	Options                   CompilerOptions
	SolverOptions             solver.Options
}

func NewLuetCompiler(backend CompilerBackend, db pkg.PackageDatabase, opt *CompilerOptions, solvopts solver.Options) Compiler {
	// The CompilerRecipe will gives us a tree with only build deps listed.
	return &LuetCompiler{
		Backend: backend,
		CompilerRecipe: &tree.CompilerRecipe{
			tree.Recipe{Database: db},
		},
		Database:        db,
		ImageRepository: opt.ImageRepository,
		PullFirst:       opt.PullFirst,
		CompressionType: opt.CompressionType,
		KeepImg:         opt.KeepImg,
		Concurrency:     opt.Concurrency,
		Options:         *opt,
		SolverOptions:   solvopts,
	}
}

func (cs *LuetCompiler) SetConcurrency(i int) {
	cs.Concurrency = i
}

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
		// for _, assertion := range a.GetSourceAssertion() {
		// 	if assertion.Value && assertion.Package.Flagged() {
		// 		spec, asserterr := cs.FromPackage(assertion.Package)
		// 		if err != nil {
		// 			return nil, append(err, asserterr)
		// 		}
		// 		w, asserterr := cs.Tree().World()
		// 		if err != nil {
		// 			return nil, append(err, asserterr)
		// 		}
		// 		revdeps := spec.GetPackage().Revdeps(&w)
		// 		for _, r := range revdeps {
		// 			spec, asserterr := cs.FromPackage(r)
		// 			if asserterr != nil {
		// 				return nil, append(err, asserterr)
		// 			}
		// 			spec.SetOutputPath(ps.All()[0].GetOutputPath())

		// 			toCompile.Add(spec)
		// 		}
		// 	}
		// }
	}

	uniques := toCompile.Unique().Remove(ps)
	for _, u := range uniques.All() {
		Info(" :arrow_right_hook:", u.GetPackage().GetName(), ":leaves:", u.GetPackage().GetVersion(), "(", u.GetPackage().GetCategory(), ")")
	}

	artifacts2, err := cs.CompileParallel(keepPermissions, uniques)
	return append(artifacts, artifacts2...), err
}

func (cs *LuetCompiler) CompileParallel(keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error) {
	Spinner(22)
	defer SpinnerStop()
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
	// TODO: As the salt contains the packageImage ( in registry/organization/imagename:tag format)
	// the images hashes are broken with registry mirrors.
	// We should use the image tag, or pass by the package assertion hash which is unique
	// and identifies the deptree of the package.

	fp := p.GetPackage().HashFingerprint(packageImage)

	if buildertaggedImage == "" {
		buildertaggedImage = cs.ImageRepository + ":builder-" + fp
		Debug(pkgTag, "Creating intermediary image", buildertaggedImage, "from", image)
	}

	// TODO:  Cleanup, not actually hit
	if packageImage == "" {
		packageImage = cs.ImageRepository + ":builder-invalid" + fp
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
	}
	runnerOpts = CompilerBackendOptions{
		ImageName:      packageImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + ".dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
	}

	buildAndPush := func(opts CompilerBackendOptions) error {
		buildImage := true
		if cs.Options.PullFirst {
			bus.Manager.Publish(bus.EventImagePrePull, opts)
			err := cs.Backend.DownloadImage(opts)
			if err == nil {
				buildImage = false
			} else {
				Warning("Failed to download '" + opts.ImageName + "'. Will keep going and build the image unless you use --fatal")
				Warning(err.Error())
			}
			bus.Manager.Publish(bus.EventImagePostPull, opts)
		}
		if buildImage {
			bus.Manager.Publish(bus.EventImagePreBuild, opts)
			if err := cs.Backend.BuildImage(opts); err != nil {
				return errors.Wrap(err, "Could not build image: "+image+" "+opts.DockerFileName)
			}
			bus.Manager.Publish(bus.EventImagePostBuild, opts)
			if cs.Options.Push {
				bus.Manager.Publish(bus.EventImagePrePush, opts)
				if err = cs.Backend.Push(opts); err != nil {
					return errors.Wrap(err, "Could not push image: "+image+" "+opts.DockerFileName)
				}
				bus.Manager.Publish(bus.EventImagePostPush, opts)
			}
		}
		return nil
	}
	if len(p.GetPreBuildSteps()) != 0 {
		Info(pkgTag, ":whale: Generating 'builder' image from", image, "as", buildertaggedImage, "with prelude steps")
		if err := buildAndPush(builderOpts); err != nil {
			return builderOpts, runnerOpts, errors.Wrap(err, "Could not push image: "+image+" "+builderOpts.DockerFileName)
		}
	}

	// Even if we might not have any steps to build, we do that so we can tag the image used in this moment and use that to cache it in a registry, or in the system.
	// acting as a docker tag.
	Info(pkgTag, ":whale: Generating 'package' image from", buildertaggedImage, "as", packageImage, "with build steps")
	if err := buildAndPush(runnerOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not push image: "+image+" "+builderOpts.DockerFileName)
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

func (cs *LuetCompiler) waitForImage(image string) {
	if cs.Options.PullFirst && cs.Options.Wait && !cs.Backend.ImageAvailable(image) {
		Info(fmt.Sprintf("Waiting for image %s to be available... :zzz:", image))
		Spinner(22)
		defer SpinnerStop()
		for !cs.Backend.ImageAvailable(image) {
			Info(fmt.Sprintf("Image %s not available yet, sleeping", image))
			time.Sleep(5 * time.Second)
		}
	}
}

func (cs *LuetCompiler) compileWithImage(image, buildertaggedImage, packageImage string,
	concurrency int,
	keepPermissions, keepImg bool,
	p CompilationSpec, generateArtifact bool) (Artifact, error) {

	// If it is a virtual, check if we have to generate an empty artifact or not.
	if generateArtifact && p.EmptyPackage() && !p.HasImageSource() {
		return cs.genArtifact(p, CompilerBackendOptions{}, CompilerBackendOptions{}, concurrency, keepPermissions)
	} else if p.EmptyPackage() && !p.HasImageSource() {
		return &PackageArtifact{}, nil
	}

	if !generateArtifact {
		exists := cs.Backend.ImageExists(packageImage)
		if art, err := LoadArtifactFromYaml(p); err == nil && exists { // If YAML is correctly loaded, and both images exists, no reason to rebuild.
			Debug("Artifact reloaded from YAML. Skipping build")
			return art, err
		}
		cs.waitForImage(packageImage)
		if cs.Options.PullFirst && cs.Backend.ImageAvailable(packageImage) {
			return &PackageArtifact{}, nil
		}
	}

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

// Compile is non-parallel
func (cs *LuetCompiler) Compile(keepPermissions bool, p CompilationSpec) (Artifact, error) {
	asserts, err := cs.ComputeDepTree(p)
	if err != nil {
		panic(err)
	}
	p.SetSourceAssertion(asserts)
	return cs.compile(cs.Concurrency, keepPermissions, p)
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
	targetPackageHash := cs.ImageRepository + ":" + targetAssertion.Hash.PackageHash

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
		return cs.compileWithImage(p.GetImage(), "", targetPackageHash, concurrency, keepPermissions, cs.KeepImg, p, true)
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

			buildImageHash := cs.ImageRepository + ":" + assertion.Hash.BuildHash
			currentPackageImageHash := cs.ImageRepository + ":" + assertion.Hash.PackageHash
			Debug(pkgTag, "    :arrow_right_hook: :whale: Builder image from", buildImageHash)
			Debug(pkgTag, "    :arrow_right_hook: :whale: Package image name", currentPackageImageHash)

			bus.Manager.Publish(bus.EventPackagePreBuild, struct {
				CompileSpec CompilationSpec
				Assert      solver.PackageAssert
			}{
				CompileSpec: compileSpec,
				Assert:      assertion,
			})

			lastHash = currentPackageImageHash
			if compileSpec.GetImage() != "" {
				Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from image")
				artifact, err := cs.compileWithImage(compileSpec.GetImage(), buildImageHash, currentPackageImageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec, packageDeps)
				if err != nil {
					return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
				}
				departifacts = append(departifacts, artifact)
				Info(pkgTag, ":white_check_mark: Done")
				continue
			}

			Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from tree")
			artifact, err := cs.compileWithImage(buildImageHash, "", currentPackageImageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec, packageDeps)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
				//	deperrs = append(deperrs, err)
				//		break // stop at first error
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
		lastHash = cs.ImageRepository + ":" + dependencies[len(dependencies)-1].Hash.PackageHash
	}

	if !cs.Options.OnlyDeps {
		Info(":rocket: All dependencies are satisfied, building package requested by the user", p.GetPackage().HumanReadableString())
		Info(":package:", p.GetPackage().HumanReadableString(), " Using image: ", lastHash)
		artifact, err := cs.compileWithImage(lastHash, "", targetPackageHash, concurrency, keepPermissions, cs.KeepImg, p, true)
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

func (cs *LuetCompiler) FromPackage(p pkg.Package) (CompilationSpec, error) {

	pack, err := cs.Database.FindPackageCandidate(p)
	if err != nil {
		return nil, err
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

		raw := packsRaw.Find(pack.GetName(), pack.GetCategory(), pack.GetVersion())

		d := map[string]interface{}{}
		if len(cs.Options.BuildValuesFile) > 0 {
			defBuild, err := ioutil.ReadFile(cs.Options.BuildValuesFile)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+val)
			}
			err = yaml.Unmarshal(defBuild, &d)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+val)
			}
		}

		dat, err := helpers.RenderHelm(string(dataBuild), raw, d)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(dat)
	} else {
		out, err := helpers.RenderFiles(pack.Rel(BuildFile), val, cs.Options.BuildValuesFile)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(out)
	}

	return NewLuetCompilationSpec(dataresult, pack)
}

func (cs *LuetCompiler) GetBackend() CompilerBackend {
	return cs.Backend
}

func (cs *LuetCompiler) SetBackend(b CompilerBackend) {
	cs.Backend = b
}
