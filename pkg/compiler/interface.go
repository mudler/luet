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
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
)

type Compiler interface {
	Compile(int, bool, CompilationSpec) (Artifact, error)
	CompileParallel(concurrency int, keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error)
	CompileWithReverseDeps(concurrency int, keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error)
	ComputeDepTree(p CompilationSpec) (solver.PackagesAssertions, error)
	Prepare(concurrency int) error

	FromPackage(pkg.Package) (CompilationSpec, error)

	SetBackend(CompilerBackend)
	GetBackend() CompilerBackend
}

type CompilerBackendOptions struct {
	ImageName      string
	SourcePath     string
	DockerFileName string
	Destination    string
}

type CompilerBackend interface {
	BuildImage(CompilerBackendOptions) error
	ExportImage(CompilerBackendOptions) error
	RemoveImage(CompilerBackendOptions) error
	Changes(fromImage, toImage string) ([]ArtifactLayer, error)
	ImageDefinitionToTar(CompilerBackendOptions) error
	ExtractRootfs(opts CompilerBackendOptions, keepPerms bool) error

	CopyImage(string, string) error
	DownloadImage(opts CompilerBackendOptions) error
}

type Artifact interface {
	GetPath() string
	SetPath(string)
	GetDependencies() []Artifact
	SetDependencies(d []Artifact)
	GetSourceAssertion() solver.PackagesAssertions
	SetSourceAssertion(as solver.PackagesAssertions)

	SetCompileSpec(as CompilationSpec)
	GetCompileSpec() CompilationSpec
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

// CompilationSpec represent a compilation specification derived from a package
type CompilationSpec interface {
	ImageUnpack() bool // tells if the definition is just an image
	GetIncludes() []string

	RenderBuildImage() (string, error)
	WriteBuildImageDefinition(string) error

	RenderStepImage(image string) (string, error)
	WriteStepImageDefinition(fromimage, path string) error

	GetPackage() pkg.Package
	BuildSteps() []string

	GetSeedImage() string
	SetSeedImage(string)

	GetImage() string
	SetImage(string)

	SetOutputPath(string)
	GetOutputPath() string
	Rel(string) string

	GetPreBuildSteps() []string

	GetSourceAssertion() solver.PackagesAssertions
	SetSourceAssertion(as solver.PackagesAssertions)
}

type CompilationSpecs interface {
	Unique() CompilationSpecs
	Len() int
	All() []CompilationSpec
	Add(CompilationSpec)
	Remove(s CompilationSpecs) CompilationSpecs
}
