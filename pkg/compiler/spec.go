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
	yaml "gopkg.in/yaml.v2"
)

type LuetCompilationSpec struct {
	Steps      []string    `json:"steps"`   // Are run inside a container and the result layer diff is saved
	Prelude    []string    `json:"prelude"` // Are run inside the image which will be our builder
	Image      string      `json:"image"`
	Seed       string      `json:"seed"`
	Package    pkg.Package `json:"-"`
	OutputPath string      `json:"-"` // Where the build processfiles go
}

func NewLuetCompilationSpec(b []byte, p pkg.Package) (CompilationSpec, error) {
	var spec LuetCompilationSpec
	err := yaml.Unmarshal(b, &spec)
	if err != nil {
		return &spec, err
	}
	spec.Package = p
	return &spec, nil
}

func (cs *LuetCompilationSpec) GetPackage() pkg.Package {
	return cs.Package
}

func (cs *LuetCompilationSpec) BuildSteps() []string {
	return cs.Steps
}

func (cs *LuetCompilationSpec) GetPreBuildSteps() []string {
	return cs.Prelude
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

// TODO: docker build image first. Then a backend can be used to actually spin up a container with it and run the steps within
func (cs *LuetCompilationSpec) RenderBuildImage() (string, error) {
	spec := `
FROM ` + cs.GetSeedImage() + `
COPY . /luetbuild
WORKDIR /luetbuild
`
	for _, s := range cs.GetPreBuildSteps() {
		spec = spec + `
RUN ` + s
	}
	return spec, nil
}

// TODO: docker build image first. Then a backend can be used to actually spin up a container with it and run the steps within
func (cs *LuetCompilationSpec) RenderStepImage(image string) (string, error) {
	spec := `
FROM ` + image
	for _, s := range cs.BuildSteps() {
		spec = spec + `
RUN ` + s
	}

	return spec, nil
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
