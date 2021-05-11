// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

	compilerspec "github.com/mudler/luet/pkg/compiler/types/spec"
	"github.com/mudler/luet/pkg/config"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"
)

type ImageHashTree struct {
	Database      pkg.PackageDatabase
	SolverOptions config.LuetSolverOptions
}

type PackageImageHashTree struct {
	Target                       *solver.PackageAssert
	Dependencies                 solver.PackagesAssertions
	Solution                     solver.PackagesAssertions
	dependencyBuilderImageHashes map[string]string
	SourceHash                   string
	BuilderImageHash             string
}

func NewHashTree(db pkg.PackageDatabase) *ImageHashTree {
	return &ImageHashTree{
		Database: db,
	}
}

func (ht *PackageImageHashTree) DependencyBuildImage(p pkg.Package) (string, error) {
	found, ok := ht.dependencyBuilderImageHashes[p.GetFingerPrint()]
	if !ok {
		return "", errors.New("package hash not found")
	}
	return found, nil
}

// TODO: ___ When computing the hash per package (and evaluating the sat solver solution tree part)
// we should use the hash of each package  + its fingerprint  instead as a salt.
// That's because the hash will be salted with its `build.yaml`.
// In this way, we trigger recompilations if some dep of a target changes
// a build.yaml, without touching the version
func (ht *ImageHashTree) Query(cs *LuetCompiler, p *compilerspec.LuetCompilationSpec) (*PackageImageHashTree, error) {
	assertions, err := ht.resolve(cs, p)
	if err != nil {
		return nil, err
	}
	targetAssertion := assertions.Search(p.GetPackage().GetFingerPrint())

	dependencies := assertions.Drop(p.GetPackage())
	var sourceHash string
	imageHashes := map[string]string{}
	for _, assertion := range dependencies {
		var depbuildImageTag string
		compileSpec, err := cs.FromPackage(assertion.Package)
		if err != nil {
			return nil, errors.Wrap(err, "Error while generating compilespec for "+assertion.Package.GetName())
		}
		if compileSpec.GetImage() != "" {
			depbuildImageTag = assertion.Hash.BuildHash
		} else {
			depbuildImageTag = ht.genBuilderImageTag(compileSpec, targetAssertion.Hash.PackageHash)
		}
		imageHashes[assertion.Package.GetFingerPrint()] = depbuildImageTag
		sourceHash = assertion.Hash.PackageHash
	}

	return &PackageImageHashTree{
		Dependencies:                 dependencies,
		Target:                       targetAssertion,
		SourceHash:                   sourceHash,
		BuilderImageHash:             ht.genBuilderImageTag(p, targetAssertion.Hash.PackageHash),
		dependencyBuilderImageHashes: imageHashes,
		Solution:                     assertions,
	}, nil
}

func (ht *ImageHashTree) genBuilderImageTag(p *compilerspec.LuetCompilationSpec, packageImage string) string {
	// Use packageImage as salt into the fp being used
	// so the hash is unique also in cases where
	// some package deps does have completely different
	// depgraphs
	return fmt.Sprintf("builder-%s", p.GetPackage().HashFingerprint(packageImage))
}

// resolve computes the dependency tree of a compilation spec and returns solver assertions
// in order to be able to compile the spec.
func (ht *ImageHashTree) resolve(cs *LuetCompiler, p *compilerspec.LuetCompilationSpec) (solver.PackagesAssertions, error) {
	dependencies, err := cs.ComputeDepTree(p)
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().HumanReadableString())
	}

	assertions := solver.PackagesAssertions{}
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value {
			nthsolution := dependencies.Cut(assertion.Package)
			assertion.Hash = solver.PackageHash{
				BuildHash:   nthsolution.HashFrom(assertion.Package),
				PackageHash: nthsolution.AssertionHash(),
			}
			assertion.Package.SetTreeDir(p.Package.GetTreeDir())
			assertions = append(assertions, assertion)
		}
	}

	return assertions, nil
}
