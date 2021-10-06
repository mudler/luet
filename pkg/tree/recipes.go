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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	pkg "github.com/mudler/luet/pkg/package"
	spectooling "github.com/mudler/luet/pkg/spectooling"

	"github.com/pkg/errors"
)

func NewGeneralRecipe(db pkg.PackageDatabase) Builder { return &Recipe{Database: db} }

// Recipe is the "general" reciper for Trees
type Recipe struct {
	SourcePath []string
	Database   pkg.PackageDatabase
}

func WriteDefinitionFile(p pkg.Package, definitionFilePath string) error {
	data, err := spectooling.NewDefaultPackageSanitized(p).Yaml()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(definitionFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (r *Recipe) Save(path string) error {
	for _, p := range r.Database.World() {
		dir := filepath.Join(path, p.GetCategory(), p.GetName(), p.GetVersion())
		os.MkdirAll(dir, os.ModePerm)

		err := WriteDefinitionFile(p, filepath.Join(dir, pkg.PackageDefinitionFile))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Recipe) Load(path string) error {

	// tmpfile, err := ioutil.TempFile("", "luet")
	// if err != nil {
	// 	return err
	// }
	if !fileHelper.Exists(path) {
		return errors.New(fmt.Sprintf(
			"Path %s doesn't exit.", path,
		))
	}

	r.SourcePath = append(r.SourcePath, path)

	if r.Database == nil {
		r.Database = pkg.NewInMemoryDatabase(false)
	}

	//r.Tree().SetPackageSet(pkg.NewBoltDatabase(tmpfile.Name()))
	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()

	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		if info.Name() != pkg.PackageDefinitionFile && info.Name() != pkg.PackageCollectionFile {
			return nil // Skip with no errors
		}

		dat, err := ioutil.ReadFile(currentpath)
		if err != nil {
			return errors.Wrap(err, "Error reading file "+currentpath)
		}

		switch info.Name() {
		case pkg.PackageDefinitionFile:
			pack, err := pkg.DefaultPackageFromYaml(dat)
			if err != nil {
				return errors.Wrap(err, "Error reading yaml "+currentpath)
			}

			// Path is set only internally when tree is loaded from disk
			pack.SetPath(filepath.Dir(currentpath))
			_, err = r.Database.CreatePackage(&pack)
			if err != nil {
				return errors.Wrap(err, "Error creating package "+pack.GetName())
			}
		case pkg.PackageCollectionFile:
			packs, err := pkg.DefaultPackagesFromYAML(dat)
			if err != nil {
				return errors.Wrap(err, "Error reading yaml "+currentpath)
			}
			for _, p := range packs {
				// Path is set only internally when tree is loaded from disk
				p.SetPath(filepath.Dir(currentpath))
				_, err = r.Database.CreatePackage(&p)
				if err != nil {
					return errors.Wrap(err, "Error creating package "+p.GetName())
				}
			}

		}

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return err
	}
	return nil
}

func (r *Recipe) GetDatabase() pkg.PackageDatabase   { return r.Database }
func (r *Recipe) WithDatabase(d pkg.PackageDatabase) { r.Database = d }
func (r *Recipe) GetSourcePath() []string            { return r.SourcePath }
