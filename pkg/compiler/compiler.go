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

	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	"github.com/pkg/errors"
)

const BuildFile = "build.yaml"

type LuetCompiler struct {
	*tree.CompilerRecipe
	Backend                   CompilerBackend
	Database                  pkg.PackageDatabase
	ImageRepository           string
	PullFirst, KeepImg, Clean bool
	Concurrency               int
	CompressionType           CompressionImplementation
	Options                   CompilerOptions
}

func NewLuetCompiler(backend CompilerBackend, db pkg.PackageDatabase, opt *CompilerOptions) Compiler {
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
		Clean:           opt.Clean,
		Options:         *opt,
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

func (cs *LuetCompiler) stripIncludesFromRootfs(includes []string, rootfs string) error {
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

		if !match {
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

func (cs *LuetCompiler) compileWithImage(image, buildertaggedImage, packageImage string, concurrency int, keepPermissions, keepImg bool, p CompilationSpec) (Artifact, error) {
	if !cs.Clean {
		if art, err := LoadArtifactFromYaml(p); err == nil {
			Debug("Artifact reloaded. Skipping build")
			return art, err
		}
	}
	pkgTag := ":package:  " + p.GetPackage().GetName()

	p.SetSeedImage(image) // In this case, we ignore the build deps as we suppose that the image has them - otherwise we recompose the tree with a solver,
	// and we build all the images first.

	err := os.MkdirAll(p.Rel("build"), os.ModePerm)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for building")
	}
	buildDir, err := ioutil.TempDir(p.Rel("build"), "pack")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for building")
	}
	defer os.RemoveAll(buildDir) // clean up

	// First we copy the source definitions into the output - we create a copy which the builds will need (we need to cache this phase somehow)
	err = helpers.CopyDir(p.GetPackage().GetPath(), buildDir)
	if err != nil {
		return nil, errors.Wrap(err, "Could not copy package sources")

	}

	// Copy file into the build context, the compilespec might have requested to do so.
	if len(p.GetRetrieve()) > 0 {
		err := p.CopyRetrieves(buildDir)
		if err != nil {
			Warning("Failed copying retrieves", err.Error())
		}
	}

	if buildertaggedImage == "" {
		buildertaggedImage = cs.ImageRepository + "-" + p.GetPackage().GetFingerPrint() + "-builder"
	}
	if packageImage == "" {
		packageImage = cs.ImageRepository + "-" + p.GetPackage().GetFingerPrint()
	}

	if cs.PullFirst {
		//Best effort pull
		cs.Backend.DownloadImage(CompilerBackendOptions{ImageName: buildertaggedImage})
		cs.Backend.DownloadImage(CompilerBackendOptions{ImageName: packageImage})
	}

	Info(pkgTag, "Generating :whale: definition for builder image from", image)

	// First we create the builder image
	p.WriteBuildImageDefinition(filepath.Join(buildDir, p.GetPackage().GetFingerPrint()+"-builder.dockerfile"))
	builderOpts := CompilerBackendOptions{
		ImageName:      buildertaggedImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + "-builder.dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + "-builder.image.tar"),
	}

	err = cs.Backend.BuildImage(builderOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not build image: "+image+" "+builderOpts.DockerFileName)
	}

	err = cs.Backend.ExportImage(builderOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not export image")
	}

	// Then we write the step image, which uses the builder one
	p.WriteStepImageDefinition(buildertaggedImage, filepath.Join(buildDir, p.GetPackage().GetFingerPrint()+".dockerfile"))
	runnerOpts := CompilerBackendOptions{
		ImageName:      packageImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().GetFingerPrint() + ".dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
	}

	// if !keepPackageImg {
	// 	err = cs.Backend.ImageDefinitionToTar(runnerOpts)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "Could not export image to tar")
	// 	}
	// } else {
	if err := cs.Backend.BuildImage(runnerOpts); err != nil {
		return nil, errors.Wrap(err, "Failed building image for "+runnerOpts.ImageName+" "+runnerOpts.DockerFileName)
	}
	if err := cs.Backend.ExportImage(runnerOpts); err != nil {
		return nil, errors.Wrap(err, "Failed exporting image")
	}
	//	}

	var diffs []ArtifactLayer
	var artifact Artifact

	if !p.ImageUnpack() {
		// we have to get diffs only if spec is not unpacked
		diffs, err = cs.Backend.Changes(p.Rel(p.GetPackage().GetFingerPrint()+"-builder.image.tar"), p.Rel(p.GetPackage().GetFingerPrint()+".image.tar"))
		if err != nil {
			return nil, errors.Wrap(err, "Could not generate changes from layers")
		}
	}

	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	// TODO: Compression and such
	err = cs.Backend.ExtractRootfs(CompilerBackendOptions{
		ImageName:  packageImage,
		SourcePath: runnerOpts.Destination, Destination: rootfs}, keepPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}

	if !keepImg {
		// We keep them around, so to not reload them from the tar (which should be the "correct way") and we automatically share the same layers
		// TODO: Handle caching and optionally do not remove things
		err = cs.Backend.RemoveImage(builderOpts)
		if err != nil {
			Warning("Could not remove image ", builderOpts.ImageName)
			//	return nil, errors.Wrap(err, "Could not remove image")
		}
		err = cs.Backend.RemoveImage(runnerOpts)
		if err != nil {
			Warning("Could not remove image ", builderOpts.ImageName)
			//	return nil, errors.Wrap(err, "Could not remove image")
		}
	}

	if p.ImageUnpack() {

		if len(p.GetIncludes()) > 0 {
			// strip from includes
			cs.stripIncludesFromRootfs(p.GetIncludes(), rootfs)
		}
		artifact = NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar"))
		artifact.SetCompressionType(cs.CompressionType)
		err = artifact.Compress(rootfs, concurrency)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating package archive")
		}

		artifact.SetCompileSpec(p)
	} else {
		Info(pkgTag, "Generating delta")

		artifact, err = ExtractArtifactFromDelta(rootfs, p.Rel(p.GetPackage().GetFingerPrint()+".package.tar"), diffs, concurrency, keepPermissions, p.GetIncludes(), cs.CompressionType)
		if err != nil {
			return nil, errors.Wrap(err, "Could not generate deltas")
		}

		artifact.SetCompileSpec(p)
	}

	err = artifact.WriteYaml(p.GetOutputPath())
	if err != nil {
		return artifact, err
	}
	Info(pkgTag, "   :white_check_mark: Done")

	return artifact, nil
}

func (cs *LuetCompiler) packageFromImage(p CompilationSpec, tag string, keepPermissions, keepImg bool, concurrency int) (Artifact, error) {
	if !cs.Clean {
		if art, err := LoadArtifactFromYaml(p); err == nil {
			Debug("Artifact reloaded. Skipping build")
			return art, err
		}
	}
	pkgTag := ":package:  " + p.GetPackage().GetName()

	Info(pkgTag, "   🍩 Build starts 🔨 🔨 🔨 ")

	builderOpts := CompilerBackendOptions{
		ImageName:   p.GetImage(),
		Destination: p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
	}
	err := cs.Backend.DownloadImage(builderOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not download image")
	}

	if tag != "" {
		err = cs.Backend.CopyImage(p.GetImage(), tag)
		if err != nil {
			return nil, errors.Wrap(err, "Could not download image")
		}
	}
	err = cs.Backend.ExportImage(builderOpts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not export image")
	}

	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	// TODO: Compression and such
	err = cs.Backend.ExtractRootfs(CompilerBackendOptions{
		ImageName:  p.GetImage(),
		SourcePath: builderOpts.Destination, Destination: rootfs}, keepPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}
	artifact := NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar"))
	artifact.SetCompileSpec(p)
	artifact.SetCompressionType(cs.CompressionType)

	err = artifact.Compress(rootfs, concurrency)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}

	if !keepImg {
		// We keep them around, so to not reload them from the tar (which should be the "correct way") and we automatically share the same layers
		// TODO: Handle caching and optionally do not remove things
		err = cs.Backend.RemoveImage(builderOpts)
		if err != nil {
			Warning("Could not remove image ", builderOpts.ImageName)
			//	return nil, errors.Wrap(err, "Could not remove image")
		}
	}

	Info(pkgTag, "   :white_check_mark: Done")

	err = artifact.WriteYaml(p.GetOutputPath())
	if err != nil {
		return artifact, err
	}
	return artifact, nil
}

func (cs *LuetCompiler) ComputeDepTree(p CompilationSpec) (solver.PackagesAssertions, error) {

	s := solver.NewResolver(pkg.NewInMemoryDatabase(false), cs.Database, pkg.NewInMemoryDatabase(false), cs.Options.SolverOptions.Resolver())

	solution, err := s.Install([]pkg.Package{p.GetPackage()})
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().GetName())
	}

	dependencies := solution.Order(cs.Database, p.GetPackage().GetFingerPrint())

	assertions := solver.PackagesAssertions{}
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value {
			nthsolution := dependencies.Cut(assertion.Package)

			assertion.Hash = solver.PackageHash{
				BuildHash:   nthsolution.Drop(assertion.Package).AssertionHash(),
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
	Info(":package: Compiling", p.GetPackage().GetName(), "version", p.GetPackage().GetVersion(), ".... :coffee:")

	if len(p.GetPackage().GetRequires()) == 0 && p.GetImage() == "" {
		Error("Package with no deps and no seed image supplied, bailing out")
		return nil, errors.New("Package " + p.GetPackage().GetFingerPrint() + "with no deps and no seed image supplied, bailing out")
	}

	// - If image is set we just generate a plain dockerfile
	// Treat last case (easier) first. The image is provided and we just compute a plain dockerfile with the images listed as above
	if p.GetImage() != "" {
		if p.ImageUnpack() { // If it is just an entire image, create a package from it
			return cs.packageFromImage(p, "", keepPermissions, cs.KeepImg, concurrency)
		}

		return cs.compileWithImage(p.GetImage(), "", "", concurrency, keepPermissions, cs.KeepImg, p)
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

	Info(":deciduous_tree: Build dependencies for " + p.GetPackage().GetName())
	for _, assertion := range dependencies { //highly dependent on the order
		depsN++
		Info(" :arrow_right_hook:", assertion.Package.GetName(), ":leaves:", assertion.Package.GetVersion(), "(", assertion.Package.GetCategory(), ")")

	}

	for _, assertion := range dependencies { //highly dependent on the order
		currentN++
		pkgTag := fmt.Sprintf(":package:  %d/%d %s ⤑ %s", currentN, depsN, p.GetPackage().GetName(), assertion.Package.GetName())
		Info(pkgTag, "   :zap:  Building dependency")
		compileSpec, err := cs.FromPackage(assertion.Package)
		if err != nil {
			return nil, errors.Wrap(err, "Error while generating compilespec for "+assertion.Package.GetName())
		}
		compileSpec.SetOutputPath(p.GetOutputPath())

		buildImageHash := cs.ImageRepository + ":" + assertion.Hash.BuildHash
		currentPackageImageHash := cs.ImageRepository + ":" + assertion.Hash.PackageHash
		Debug(pkgTag, "    :arrow_right_hook: :whale: Builder image from", buildImageHash)
		Debug(pkgTag, "    :arrow_right_hook: :whale: Package image name", currentPackageImageHash)

		lastHash = currentPackageImageHash
		if compileSpec.GetImage() != "" {
			// TODO: Refactor this
			if compileSpec.ImageUnpack() { // If it is just an entire image, create a package from it
				if compileSpec.GetImage() == "" {
					return nil, errors.New("No image defined for package: " + assertion.Package.GetName())
				}
				Info(pkgTag, ":whale: Sourcing package from image", compileSpec.GetImage())
				artifact, err := cs.packageFromImage(compileSpec, currentPackageImageHash, keepPermissions, cs.KeepImg, concurrency)
				if err != nil {
					return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().GetName())
				}
				departifacts = append(departifacts, artifact)
				continue
			}

			Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().GetFingerPrint()+" from image")
			artifact, err := cs.compileWithImage(compileSpec.GetImage(), buildImageHash, currentPackageImageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().GetName())
			}
			departifacts = append(departifacts, artifact)
			Info(pkgTag, ":white_check_mark: Done")
			continue
		}

		Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().GetFingerPrint()+" from tree")
		artifact, err := cs.compileWithImage(buildImageHash, "", currentPackageImageHash, concurrency, keepPermissions, cs.KeepImg, compileSpec)
		if err != nil {
			return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().GetName())
			//	deperrs = append(deperrs, err)
			//		break // stop at first error
		}
		departifacts = append(departifacts, artifact)
		Info(pkgTag, ":collision: Done")
	}

	Info(":package:", p.GetPackage().GetName(), ":cyclone:  Building package target from:", lastHash)
	artifact, err := cs.compileWithImage(lastHash, "", "", concurrency, keepPermissions, cs.KeepImg, p)
	if err != nil {
		return artifact, err
	}
	artifact.SetDependencies(departifacts)
	artifact.SetSourceAssertion(p.GetSourceAssertion())

	return artifact, err
}

func (cs *LuetCompiler) FromPackage(p pkg.Package) (CompilationSpec, error) {

	pack, err := cs.Database.FindPackageCandidate(p)
	if err != nil {
		return nil, err
	}

	buildFile := pack.Rel(BuildFile)
	if !helpers.Exists(buildFile) {
		return nil, errors.New("No build file present for " + p.GetFingerPrint())
	}

	dat, err := ioutil.ReadFile(buildFile)
	if err != nil {
		return nil, err
	}
	return NewLuetCompilationSpec(dat, pack)
}

func (cs *LuetCompiler) GetBackend() CompilerBackend {
	return cs.Backend
}

func (cs *LuetCompiler) SetBackend(b CompilerBackend) {
	cs.Backend = b
}
