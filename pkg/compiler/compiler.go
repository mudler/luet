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
	"errors"
	"io/ioutil"

	"github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
)

const BuildFile = "build.yaml"

type LuetCompiler struct {
	*tree.Recipe
	Backend CompilerBackend
}

func NewLuetCompiler(backend CompilerBackend, t pkg.Tree) Compiler {
	return &LuetCompiler{Backend: backend, Recipe: &tree.Recipe{PackageTree: t}}
}

func (cs *LuetCompiler) Compile(p CompilationSpec) (*Artifact, error) {
	return nil, errors.New("Not implemented yet")
}

func (cs *LuetCompiler) FromPackage(p pkg.Package) (CompilationSpec, error) {

	pack, err := cs.Tree().GetPackageSet().FindPackage(p)
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
	return NewLuetCompilationSpec(dat)
}
