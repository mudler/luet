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

// Recipe is a builder imeplementation.

// It reads a Tree and spit it in human readable form (YAML), called recipe,
// It also loads a tree (recipe) from a YAML (to a db, e.g. BoltDB), allowing to query it
// with the solver, using the package object.
package tree

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/pkg/errors"
)

const (
	CompilerDefinitionFile = "build.yaml"
)

func NewCompilerRecipe(d pkg.PackageDatabase) Builder {
	return &CompilerRecipe{Recipe: Recipe{Database: d}}
}

// Recipe is the "general" reciper for Trees
type CompilerRecipe struct {
	Recipe
}

func (r *CompilerRecipe) Load(path string) error {

	if r.Tree() == nil {
		r.PackageTree = NewDefaultTree()
	}
	r.SourcePath = path
	//tmpfile, err := ioutil.TempFile("", "luet")
	//if err != nil {
	//	return err
	//}

	r.Tree().SetPackageSet(r.Database)
	//r.Tree().SetPackageSet(pkg.NewBoltDatabase(tmpfile.Name()))
	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()
	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		if info.Name() != DefinitionFile {
			return nil // Skip with no errors
		}

		dat, err := ioutil.ReadFile(currentpath)
		if err != nil {
			return errors.Wrap(err, "Error reading file "+currentpath)
		}
		pack, err := pkg.DefaultPackageFromYaml(dat)
		if err != nil {
			return errors.Wrap(err, "Error reading yaml "+currentpath)
		}

		// Path is set only internally when tree is loaded from disk
		pack.SetPath(filepath.Dir(currentpath))

		// Instead of rdeps, have a different tree for build deps.
		compileDefPath := pack.Rel(CompilerDefinitionFile)
		if helpers.Exists(compileDefPath) {
			dat, err = ioutil.ReadFile(compileDefPath)
			if err != nil {
				return errors.Wrap(err, "Error reading file "+currentpath)
			}
			packbuild, err := pkg.DefaultPackageFromYaml(dat)
			if err != nil {
				return errors.Wrap(err, "Error reading yaml "+currentpath)
			}
			pack.Requires(packbuild.GetRequires())
			pack.Conflicts(packbuild.GetConflicts())
		}

		_, err = r.Tree().GetPackageSet().CreatePackage(&pack)
		if err != nil {
			return errors.Wrap(err, "Error creating package "+pack.GetName())
		}

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return err
	}
	return nil
}

func (r *CompilerRecipe) Tree() pkg.Tree        { return r.PackageTree }
func (r *CompilerRecipe) WithTree(t pkg.Tree)   { r.PackageTree = t }
func (r *CompilerRecipe) GetSourcePath() string { return r.SourcePath }
