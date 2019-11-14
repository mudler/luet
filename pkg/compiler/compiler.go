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
	"sync"

	. "github.com/mudler/luet/pkg/logger"

	"github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"
	"github.com/pkg/errors"
)

const BuildFile = "build.yaml"

type LuetCompiler struct {
	*tree.CompilerRecipe
	Backend CompilerBackend
}

func NewLuetCompiler(backend CompilerBackend, t pkg.Tree) Compiler {
	// The CompilerRecipe will gives us a tree with only build deps listed.
	return &LuetCompiler{
		Backend: backend,
		CompilerRecipe: &tree.CompilerRecipe{
			tree.Recipe{PackageTree: t},
		},
	}
}

func (cs *LuetCompiler) compilerWorker(i int, wg *sync.WaitGroup, cspecs chan CompilationSpec, a *[]Artifact, m *sync.Mutex, concurrency int, keepPermissions bool, errors chan error) {
	defer wg.Done()

	for s := range cspecs {
		ar, err := cs.Compile(concurrency, keepPermissions, s)
		if err != nil {
			errors <- err
		}

		m.Lock()
		*a = append(*a, ar)
		m.Unlock()
	}
}

func (cs *LuetCompiler) CompileParallel(concurrency int, keepPermissions bool, ps []CompilationSpec) ([]Artifact, []error) {
	all := make(chan CompilationSpec)
	artifacts := []Artifact{}
	mutex := &sync.Mutex{}
	errors := make(chan error, len(ps))
	var wg = new(sync.WaitGroup)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go cs.compilerWorker(i, wg, all, &artifacts, mutex, concurrency, keepPermissions, errors)
	}

	for _, p := range ps {
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

func (cs *LuetCompiler) compileWithImage(image, buildertaggedImage, packageImage string, concurrency int, keepPermissions bool, p CompilationSpec) (Artifact, error) {
	p.SetSeedImage(image) // In this case, we ignore the build deps as we suppose that the image has them - otherwise we recompose the tree with a solver,
	// and we build all the images first.
	keepImg := true
	keepPackageImg := true
	buildDir := p.Rel("build")

	// First we copy the source definitions into the output - we create a copy which the builds will need (we need to cache this phase somehow)
	err := helpers.CopyDir(p.GetPackage().GetPath(), buildDir)
	if err != nil {
		return nil, errors.Wrap(err, "Could not copy package sources")

	}
	if buildertaggedImage == "" {
		keepImg = false
		buildertaggedImage = "luet/" + p.GetPackage().GetFingerPrint() + "-builder"
	}
	if packageImage == "" {
		keepPackageImg = false
		packageImage = "luet/" + p.GetPackage().GetFingerPrint()
	}

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

	if !keepPackageImg {
		err = cs.Backend.ImageDefinitionToTar(runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Could not export image to tar")
		}
	} else {
		if err := cs.Backend.BuildImage(runnerOpts); err != nil {
			return nil, errors.Wrap(err, "Failed building image for "+runnerOpts.ImageName+" "+runnerOpts.DockerFileName)
		}
		if err := cs.Backend.ExportImage(runnerOpts); err != nil {
			return nil, errors.Wrap(err, "Failed exporting image")
		}
	}

	diffs, err := cs.Backend.Changes(p.Rel(p.GetPackage().GetFingerPrint()+"-builder.image.tar"), p.Rel(p.GetPackage().GetFingerPrint()+".image.tar"))
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate changes from layers")
	}

	if !keepImg {
		// We keep them around, so to not reload them from the tar (which should be the "correct way") and we automatically share the same layers
		// TODO: Handle caching and optionally do not remove things
		err = cs.Backend.RemoveImage(builderOpts)
		if err != nil {
			// TODO: Have a --fatal flag which enables Warnings to exit.
			Warning("Could not remove image ", builderOpts.ImageName)
			//	return nil, errors.Wrap(err, "Could not remove image")
		}
	}
	rootfs, err := ioutil.TempDir(p.GetOutputPath(), "rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs) // clean up

	// TODO: Compression and such
	err = cs.Backend.ExtractRootfs(CompilerBackendOptions{SourcePath: runnerOpts.Destination, Destination: rootfs}, keepPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}
	artifact, err := ExtractArtifactFromDelta(rootfs, p.Rel(p.GetPackage().GetFingerPrint()+".package.tar"), diffs, concurrency, keepPermissions, p.GetIncludes())
	if err != nil {
		return nil, errors.Wrap(err, "Could not generate deltas")
	}

	return artifact, nil
}

func (cs *LuetCompiler) packageFromImage(p CompilationSpec, tag string, keepPermissions bool) (Artifact, error) {
	pkgTag := "📦  " + p.GetPackage().GetName()

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
	err = cs.Backend.ExtractRootfs(CompilerBackendOptions{SourcePath: builderOpts.Destination, Destination: rootfs}, keepPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "Could not extract rootfs")
	}

	err = helpers.Tar(rootfs, p.Rel(p.GetPackage().GetFingerPrint()+".package.tar"))
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}

	Info(pkgTag, "   🎉 Done")
	return NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar")), nil
}

func (cs *LuetCompiler) Compile(concurrency int, keepPermissions bool, p CompilationSpec) (Artifact, error) {
	Info("📦 Compiling", p.GetPackage().GetName(), "version", p.GetPackage().GetVersion(), ".... ☕")

	err := cs.Tree().ResolveDeps(concurrency) // FIXME: When done in parallel, this could be done on top before starting
	if err != nil {
		return nil, errors.Wrap(err, "While resoolving tree world deps")
	}

	if len(p.GetPackage().GetRequires()) == 0 && p.GetImage() == "" {
		Error("Package with no deps and no seed image supplied, bailing out")
		return nil, errors.New("Package " + p.GetPackage().GetFingerPrint() + "with no deps and no seed image supplied, bailing out")
	}

	// - If image is set we just generate a plain dockerfile

	// Treat last case (easier) first. The image is provided and we just compute a plain dockerfile with the images listed as above
	if p.GetImage() != "" {
		if p.ImageUnpack() { // If it is just an entire image, create a package from it
			return cs.packageFromImage(p, "", keepPermissions)
		}

		return cs.compileWithImage(p.GetImage(), "", "", concurrency, keepPermissions, p)
	}

	// - If image is not set, we read a base_image. Then we will build one image from it to kick-off our build based
	// on how we compute the resolvable tree.
	// This means to recursively build all the build-images needed to reach that tree part.
	// - We later on compute an hash used to identify the image, so each similar deptree keeps the same build image.

	// Get build deps tree (ordered)
	world, err := cs.Tree().World()
	if err != nil {
		return nil, errors.Wrap(err, "While computing tree world")
	}

	s := solver.NewSolver([]pkg.Package{}, world)
	pack, err := cs.Tree().FindPackage(p.GetPackage())
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().GetName())
	}
	solution, err := s.Install([]pkg.Package{pack})
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().GetName())
	}

	dependencies := solution.Order(p.GetPackage().GetFingerPrint()).Drop(p.GetPackage()) // at this point we should have a flattened list of deps to build, including all of them (with all constraints propagated already)
	departifacts := []Artifact{}                                                         // TODO: Return this somehow
	deperrs := []error{}
	var lastHash string
	depsN := 0
	currentN := 0

	Info("🌲 Build dependencies for " + p.GetPackage().GetName())
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value && assertion.Package.Flagged() {
			depsN++
			Info(" ⤷", assertion.Package.GetName(), "🍃", assertion.Package.GetVersion(), "(", assertion.Package.GetCategory(), ")")
		}
	}

	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value && assertion.Package.Flagged() {
			currentN++
			pkgTag := fmt.Sprintf("📦  %d/%d %s ⤑ %s", currentN, depsN, p.GetPackage().GetName(), assertion.Package.GetName())
			Info(pkgTag, "   🏗  Building dependency")
			compileSpec, err := cs.FromPackage(assertion.Package)
			if err != nil {
				return nil, errors.New("Error while generating compilespec for " + assertion.Package.GetName())
			}
			compileSpec.SetOutputPath(p.GetOutputPath())
			depPack, err := cs.Tree().FindPackage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().GetName())
			}

			// Generate image name of builder image - it should match with the hash up to this point - package
			nthsolution, err := s.Install([]pkg.Package{depPack})
			if err != nil {
				return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().GetName())
			}

			buildImageHash := "luet/cache:" + nthsolution.Order(depPack.GetFingerPrint()).Drop(depPack).AssertionHash()
			currentPackageImageHash := "luet/cache:" + nthsolution.Order(depPack.GetFingerPrint()).AssertionHash()
			Debug(pkgTag, "    ⤷ 🐋 Builder image name", buildImageHash)
			Debug(pkgTag, "    ⤷ 🐋 Package image name", currentPackageImageHash)

			lastHash = currentPackageImageHash
			if compileSpec.GetImage() != "" {
				// TODO: Refactor this
				if p.ImageUnpack() { // If it is just an entire image, create a package from it
					artifact, err := cs.packageFromImage(p, currentPackageImageHash, keepPermissions)
					if err != nil {
						deperrs = append(deperrs, err)
						break // stop at first error
					}
					departifacts = append(departifacts, artifact)
					continue
				}

				Debug(pkgTag, " 🍰 Compiling "+compileSpec.GetPackage().GetFingerPrint()+" from image 🐋")
				artifact, err := cs.compileWithImage(compileSpec.GetImage(), buildImageHash, currentPackageImageHash, concurrency, keepPermissions, compileSpec)
				if err != nil {
					deperrs = append(deperrs, err)
					break // stop at first error
				}
				departifacts = append(departifacts, artifact)
				Info(pkgTag, "💥 Done")
				continue
			}

			artifact, err := cs.compileWithImage(buildImageHash, "", currentPackageImageHash, concurrency, keepPermissions, compileSpec)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().GetName())
				//	deperrs = append(deperrs, err)
				//		break // stop at first error
			}
			departifacts = append(departifacts, artifact)
			Info(pkgTag, "💥 Done")
		}
	}
	Info("📦", p.GetPackage().GetName(), "🌪  Building package target from:", lastHash)

	return cs.compileWithImage(lastHash, "", "", concurrency, keepPermissions, p)
}

func (cs *LuetCompiler) FromPackage(p pkg.Package) (CompilationSpec, error) {

	pack, err := cs.Tree().FindPackage(p)
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
