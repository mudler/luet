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
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/pkg/errors"
)

const (
	CompilerDefinitionFile = "build.yaml"
)

func NewCompilerRecipe(d pkg.PackageDatabase) Builder {
	return &CompilerRecipe{Recipe: Recipe{Database: d}}
}

func ReadDefinitionFile(path string) (pkg.DefaultPackage, error) {
	empty := pkg.DefaultPackage{}
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return empty, errors.Wrap(err, "Error reading file "+path)
	}
	pack, err := pkg.DefaultPackageFromYaml(dat)
	if err != nil {
		return empty, errors.Wrap(err, "Error reading yaml "+path)
	}

	return pack, nil
}

// Recipe is the "general" reciper for Trees
type CompilerRecipe struct {
	Recipe
}

// CompilerRecipes copies tree 1:1 as they contain the specs
// and the build context required for reproducible builds
func (r *CompilerRecipe) Save(path string) error {
	for _, p := range r.SourcePath {
		if err := fileHelper.CopyDir(p, filepath.Join(path, filepath.Base(p))); err != nil {
			return errors.Wrap(err, "while copying source tree")
		}
	}
	return nil
}

func (r *CompilerRecipe) Load(path string) error {

	r.SourcePath = append(r.SourcePath, path)
	//tmpfile, err := ioutil.TempFile("", "luet")
	//if err != nil {
	//	return err
	//}
	c, err := helpers.ChartFiles([]string{filepath.Join(path, "templates")})
	if err != nil {
		return err
	}

	//r.Tree().SetPackageSet(pkg.NewBoltDatabase(tmpfile.Name()))
	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()
	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		if err != nil {
			return errors.Wrap(err, "Error on walk path "+currentpath)
		}

		if info.Name() != pkg.PackageDefinitionFile && info.Name() != pkg.PackageCollectionFile {
			return nil // Skip with no errors
		}

		switch info.Name() {
		case pkg.PackageDefinitionFile:

			pack, err := ReadDefinitionFile(currentpath)
			if err != nil {
				return err
			}
			// Path is set only internally when tree is loaded from disk
			pack.SetPath(filepath.Dir(currentpath))
			pack.SetTreeDir(path)

			// Instead of rdeps, have a different tree for build deps.
			compileDefPath := pack.Rel(CompilerDefinitionFile)
			if fileHelper.Exists(compileDefPath) {
				dat, err := helpers.RenderFiles(append(c, helpers.ChartFile(compileDefPath)...), currentpath)
				if err != nil {
					return errors.Wrap(err,
						"Error templating file "+CompilerDefinitionFile+" from "+
							filepath.Dir(currentpath))
				}

				packbuild, err := pkg.DefaultPackageFromYaml([]byte(dat))
				if err != nil {
					return errors.Wrap(err,
						"Error reading yaml "+CompilerDefinitionFile+" from "+
							filepath.Dir(currentpath))
				}
				pack.Requires(packbuild.GetRequires())
				pack.Conflicts(packbuild.GetConflicts())
			}

			_, err = r.Database.CreatePackage(&pack)
			if err != nil {
				return errors.Wrap(err, "Error creating package "+pack.GetName())
			}

		case pkg.PackageCollectionFile:

			dat, err := ioutil.ReadFile(currentpath)
			if err != nil {
				return errors.Wrap(err, "Error reading file "+currentpath)
			}

			packs, err := pkg.DefaultPackagesFromYAML(dat)
			if err != nil {
				return errors.Wrap(err, "Error reading yaml "+currentpath)
			}

			packsRaw, err := pkg.GetRawPackages(dat)
			if err != nil {
				return errors.Wrap(err, "Error reading raw packages from "+currentpath)
			}

			for _, pack := range packs {
				pack.SetPath(filepath.Dir(currentpath))
				pack.SetTreeDir(path)

				// Instead of rdeps, have a different tree for build deps.
				compileDefPath := pack.Rel(CompilerDefinitionFile)
				if fileHelper.Exists(compileDefPath) {

					raw := packsRaw.Find(pack.GetName(), pack.GetCategory(), pack.GetVersion())
					buildyaml, err := ioutil.ReadFile(compileDefPath)
					if err != nil {
						return errors.Wrap(err, "Error reading file "+currentpath)
					}
					dat, err := helpers.RenderHelm(append(c, helpers.ChartFileB(buildyaml)...), raw, map[string]interface{}{})
					if err != nil {
						return errors.Wrap(err,
							"Error templating file "+CompilerDefinitionFile+" from "+
								filepath.Dir(currentpath))
					}

					packbuild, err := pkg.DefaultPackageFromYaml([]byte(dat))
					if err != nil {
						return errors.Wrap(err,
							"Error reading yaml "+CompilerDefinitionFile+" from "+
								filepath.Dir(currentpath))
					}
					pack.Requires(packbuild.GetRequires())
					pack.Conflicts(packbuild.GetConflicts())
				}

				_, err = r.Database.CreatePackage(&pack)
				if err != nil {
					return errors.Wrap(err, "Error creating package "+pack.GetName())
				}
			}
		}
		return nil
	}

	err = filepath.Walk(path, ff)
	if err != nil {
		return err
	}
	return nil
}

func (r *CompilerRecipe) GetDatabase() pkg.PackageDatabase   { return r.Database }
func (r *CompilerRecipe) WithDatabase(d pkg.PackageDatabase) { r.Database = d }
func (r *CompilerRecipe) GetSourcePath() []string            { return r.SourcePath }
