// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package helpers

import (
	"fmt"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	pkg "github.com/mudler/luet/pkg/package"
)

func ParsePackageStr(p string) (*pkg.DefaultPackage, error) {
	gp, err := _gentoo.ParsePackageStr(p)
	if err != nil {
		return nil, err
	}
	if gp.Version == "" {
		gp.Version = "0"
		gp.Condition = _gentoo.PkgCondGreaterEqual
	}

	pkgVersion := ""
	if gp.VersionBuild != "" {
		pkgVersion = fmt.Sprintf("%s%s%s+%s",
			pkg.PkgSelectorConditionFromInt(gp.Condition.Int()).String(),
			gp.Version,
			gp.VersionSuffix,
			gp.VersionBuild,
		)
	} else {
		pkgVersion = fmt.Sprintf("%s%s%s",
			pkg.PkgSelectorConditionFromInt(gp.Condition.Int()).String(),
			gp.Version,
			gp.VersionSuffix,
		)
	}

	pack := &pkg.DefaultPackage{
		Name:     gp.Name,
		Category: gp.Category,
		Version:  pkgVersion,
		Uri:      make([]string, 0),
	}

	return pack, nil
}
