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
	"io/ioutil"
	"path/filepath"

	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/otiai10/copy"
	yaml "gopkg.in/yaml.v2"
)

type LuetCompilationspecs []LuetCompilationSpec

func NewLuetCompilationspecs(s ...CompilationSpec) CompilationSpecs {
	all := LuetCompilationspecs{}

	for _, spec := range s {
		all.Add(spec)
	}
	return &all
}

func (specs LuetCompilationspecs) Len() int {
	return len(specs)
}

func (specs *LuetCompilationspecs) Remove(s CompilationSpecs) CompilationSpecs {
	newSpecs := LuetCompilationspecs{}
SPECS:
	for _, spec := range specs.All() {
		for _, target := range s.All() {
			if target.GetPackage().Matches(spec.GetPackage()) {
				continue SPECS
			}
		}
		newSpecs.Add(spec)
	}
	return &newSpecs
}

func (specs *LuetCompilationspecs) Add(s CompilationSpec) {
	c, ok := s.(*LuetCompilationSpec)
	if !ok {
		panic("LuetCompilationspecs supports only []LuetCompilationSpec")
	}
	*specs = append(*specs, *c)
}

func (specs *LuetCompilationspecs) All() []CompilationSpec {
	var cspecs []CompilationSpec
	for i, _ := range *specs {
		f := (*specs)[i]
		cspecs = append(cspecs, &f)
	}

	return cspecs
}

func (specs *LuetCompilationspecs) Unique() CompilationSpecs {
	newSpecs := LuetCompilationspecs{}
	seen := map[string]bool{}

	for i, _ := range *specs {
		j := (*specs)[i]
		_, ok := seen[j.GetPackage().GetFingerPrint()]
		if !ok {
			seen[j.GetPackage().GetFingerPrint()] = true
			newSpecs = append(newSpecs, j)
		}
	}
	return &newSpecs
}

type LuetCompilationSpec struct {
	Steps           []string                  `json:"steps"` // Are run inside a container and the result layer diff is saved
	Env             []string                  `json:"env"`
	Prelude         []string                  `json:"prelude"` // Are run inside the image which will be our builder
	Image           string                    `json:"image"`
	Seed            string                    `json:"seed"`
	Package         *pkg.DefaultPackage       `json:"package"`
	SourceAssertion solver.PackagesAssertions `json:"-"`
	PackageDir      string                    `json:"package_dir" yaml:"package_dir"`

	Retrieve []string `json:"retrieve"`

	OutputPath string   `json:"-"` // Where the build processfiles go
	Unpack     bool     `json:"unpack"`
	Includes   []string `json:"includes"`
	Excludes   []string `json:"excludes"`
}

func NewLuetCompilationSpec(b []byte, p pkg.Package) (CompilationSpec, error) {
	var spec LuetCompilationSpec
	err := yaml.Unmarshal(b, &spec)
	if err != nil {
		return &spec, err
	}
	spec.Package = p.(*pkg.DefaultPackage)
	return &spec, nil
}
func (a *LuetCompilationSpec) GetSourceAssertion() solver.PackagesAssertions {
	return a.SourceAssertion
}

func (a *LuetCompilationSpec) SetSourceAssertion(as solver.PackagesAssertions) {
	a.SourceAssertion = as
}
func (cs *LuetCompilationSpec) GetPackage() pkg.Package {
	return cs.Package
}

func (cs *LuetCompilationSpec) GetPackageDir() string {
	return cs.PackageDir
}

func (cs *LuetCompilationSpec) SetPackageDir(s string) {
	cs.PackageDir = s
}

func (cs *LuetCompilationSpec) BuildSteps() []string {
	return cs.Steps
}

func (cs *LuetCompilationSpec) ImageUnpack() bool {
	return cs.Unpack
}

func (cs *LuetCompilationSpec) GetPreBuildSteps() []string {
	return cs.Prelude
}

func (cs *LuetCompilationSpec) GetIncludes() []string {
	return cs.Includes
}

func (cs *LuetCompilationSpec) GetExcludes() []string {
	return cs.Excludes
}

func (cs *LuetCompilationSpec) GetRetrieve() []string {
	return cs.Retrieve
}

func (cs *LuetCompilationSpec) GetSeedImage() string {
	return cs.Seed
}

func (cs *LuetCompilationSpec) GetImage() string {
	return cs.Image
}

func (cs *LuetCompilationSpec) GetOutputPath() string {
	return cs.OutputPath
}

func (p *LuetCompilationSpec) Rel(s string) string {
	return filepath.Join(p.GetOutputPath(), s)
}

func (cs *LuetCompilationSpec) SetImage(s string) {
	cs.Image = s
}

func (cs *LuetCompilationSpec) SetOutputPath(s string) {
	cs.OutputPath = s
}

func (cs *LuetCompilationSpec) SetSeedImage(s string) {
	cs.Seed = s
}

func (cs *LuetCompilationSpec) EmptyPackage() bool {
	return len(cs.BuildSteps()) == 0 && len(cs.GetPreBuildSteps()) == 0 && !cs.UnpackedPackage()
}

func (cs *LuetCompilationSpec) UnpackedPackage() bool {
	// If package_dir was specified in the spec, we want to treat the content of the directory
	// as the root of our archive.  ImageUnpack is implied to be true. override it
	unpack := cs.ImageUnpack()
	if cs.GetPackageDir() != "" {
		unpack = true
	}
	return unpack
}

func (cs *LuetCompilationSpec) HasImageSource() bool {
	return len(cs.GetPackage().GetRequires()) != 0 || cs.GetImage() != ""
}

func (cs *LuetCompilationSpec) CopyRetrieves(dest string) error {
	var err error
	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			matches, err := filepath.Glob(cs.Rel(s))

			if err != nil {
				continue
			}

			for _, m := range matches {
				err = copy.Copy(m, filepath.Join(dest, filepath.Base(m)))
			}
		}
	}
	return err
}

func (cs *LuetCompilationSpec) genDockerfile(image string, steps []string) string {
	spec := `
FROM ` + image + `
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=` + cs.Package.GetName() + `
ENV PACKAGE_VERSION=` + cs.Package.GetVersion() + `
ENV PACKAGE_CATEGORY=` + cs.Package.GetCategory()

	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			//var file string
			// if helpers.IsValidUrl(s) {
			// 	file = s
			// } else {
			// 	file = cs.Rel(s)
			// }
			spec = spec + `
ADD ` + s + ` /luetbuild/`
		}
	}

	for _, s := range cs.Env {
		spec = spec + `
ENV ` + s
	}

	for _, s := range steps {
		spec = spec + `
RUN ` + s
	}
	return spec
}

// RenderBuildImage renders the dockerfile of the image used as a pre-build step
func (cs *LuetCompilationSpec) RenderBuildImage() (string, error) {
	return cs.genDockerfile(cs.GetSeedImage(), cs.GetPreBuildSteps()), nil

}

// RenderStepImage renders the dockerfile used for the image used for building the package
func (cs *LuetCompilationSpec) RenderStepImage(image string) (string, error) {
	return cs.genDockerfile(image, cs.BuildSteps()), nil
}

func (cs *LuetCompilationSpec) WriteBuildImageDefinition(path string) error {
	data, err := cs.RenderBuildImage()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, []byte(data), 0644)
}

func (cs *LuetCompilationSpec) WriteStepImageDefinition(fromimage, path string) error {
	data, err := cs.RenderStepImage(fromimage)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, []byte(data), 0644)
}
