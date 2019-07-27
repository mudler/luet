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

package gentoo

// NOTE: Look here as an example of the builder definition executor
// https://gist.github.com/adnaan/6ca68c7985c6f851def3

import (
	"os"
	"path/filepath"
	"strings"

	tree "github.com/mudler/luet/pkg/tree"

	pkg "github.com/mudler/luet/pkg/package"
)

func NewGentooBuilder(e EbuildParser) tree.Parser {
	return &GentooBuilder{EbuildParser: e}
}

type GentooBuilder struct{ EbuildParser EbuildParser }

type GentooTree struct{ Packages pkg.PackageSet }

type EbuildParser interface {
	ScanEbuild(path string) ([]pkg.Package, error)
}

func (gt *GentooTree) GetPackageSet() pkg.PackageSet {
	return gt.Packages
}

func (gb *GentooBuilder) Generate(dir string) (pkg.Tree, error) {
	tree := &GentooTree{Packages: pkg.NewPackages([]pkg.Package{})}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), "ebuild") {
			pkgs, err := gb.EbuildParser.ScanEbuild(path)
			if err != nil {
				return err
			}
			tree.Packages.AddPackages(pkgs)

		}
		return nil
	})
	if err != nil {
		return tree, err
	}
	return tree, nil
}
