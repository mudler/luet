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
	"runtime"

	"github.com/mudler/luet/pkg/config"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
)

type Compiler interface {
	Compile(bool, CompilationSpec) (Artifact, error)
	CompileParallel(keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error)
	CompileWithReverseDeps(keepPermissions bool, ps CompilationSpecs) ([]Artifact, []error)
	ComputeDepTree(p CompilationSpec) (solver.PackagesAssertions, error)
	ComputeMinimumCompilableSet(p ...CompilationSpec) ([]CompilationSpec, error)
	SetConcurrency(i int)
	FromPackage(pkg.Package) (CompilationSpec, error)
	FromDatabase(db pkg.PackageDatabase, minimum bool, dst string) ([]CompilationSpec, error)
	SetBackend(CompilerBackend)
	GetBackend() CompilerBackend
	SetCompressionType(t CompressionImplementation)
}

type CompilerBackendOptions struct {
	ImageName      string
	SourcePath     string
	DockerFileName string
	Destination    string
	Context        string
}

type CompilerOptions struct {
	ImageRepository          string
	PullFirst, KeepImg, Push bool
	Concurrency              int
	CompressionType          CompressionImplementation

	Wait            bool
	OnlyDeps        bool
	NoDeps          bool
	SolverOptions   config.LuetSolverOptions
	BuildValuesFile string

	PackageTargetOnly bool
}

func NewDefaultCompilerOptions() *CompilerOptions {
	return &CompilerOptions{
		ImageRepository: "luet/cache",
		PullFirst:       false,
		Push:            false,
		CompressionType: None,
		KeepImg:         true,
		Concurrency:     runtime.NumCPU(),
		OnlyDeps:        false,
		NoDeps:          false,
	}
}

type CompilerBackend interface {
	BuildImage(CompilerBackendOptions) error
	ExportImage(CompilerBackendOptions) error
	RemoveImage(CompilerBackendOptions) error
	Changes(fromImage, toImage CompilerBackendOptions) ([]ArtifactLayer, error)
	ImageDefinitionToTar(CompilerBackendOptions) error
	ExtractRootfs(opts CompilerBackendOptions, keepPerms bool) error

	CopyImage(string, string) error
	DownloadImage(opts CompilerBackendOptions) error

	Push(opts CompilerBackendOptions) error
	ImageAvailable(string) bool

	ImageExists(string) bool
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
	WriteYaml(dst string) error
	Unpack(dst string, keepPerms bool) error
	Compress(src string, concurrency int) error
	SetCompressionType(t CompressionImplementation)
	FileList() ([]string, error)
	Hash() error
	Verify() error

	SetFiles(f []string)
	GetFiles() []string
	GetFileName() string

	GetChecksums() Checksums
	SetChecksums(c Checksums)

	GenerateFinalImage(string, CompilerBackend, bool) (CompilerBackendOptions, error)
	GetUncompressedName() string
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
type ArtifactLayerSummary struct {
	FromImage   string `json:"image1"`
	ToImage     string `json:"image2"`
	AddFiles    int    `json:"add_files"`
	AddSizes    int64  `json:"add_sizes"`
	DelFiles    int    `json:"del_files"`
	DelSizes    int64  `json:"del_sizes"`
	ChangeFiles int    `json:"change_files"`
	ChangeSizes int64  `json:"change_sizes"`
}
type ArtifactLayersSummary struct {
	Layers []ArtifactLayerSummary `json:"summary"`
}

// CompilationSpec represent a compilation specification derived from a package
type CompilationSpec interface {
	ImageUnpack() bool // tells if the definition is just an image
	GetIncludes() []string
	GetExcludes() []string

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

	GetRetrieve() []string
	CopyRetrieves(dest string) error

	SetPackageDir(string)
	GetPackageDir() string

	EmptyPackage() bool
	UnpackedPackage() bool
	HasImageSource() bool
}

type CompilationSpecs interface {
	Unique() CompilationSpecs
	Len() int
	All() []CompilationSpec
	Add(CompilationSpec)
	Remove(s CompilationSpecs) CompilationSpecs
}
