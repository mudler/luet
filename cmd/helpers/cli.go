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

package cmd_helpers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	. "github.com/mudler/luet/pkg/logger"

	_gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	pkg "github.com/mudler/luet/pkg/package"
	version "github.com/mudler/luet/pkg/versioner"
)

func CreateRegexArray(rgx []string) ([]*regexp.Regexp, error) {
	ans := make([]*regexp.Regexp, len(rgx))
	if len(rgx) > 0 {
		for idx, reg := range rgx {
			re := regexp.MustCompile(reg)
			if re == nil {
				return nil, errors.New("Invalid regex " + reg + "!")
			}
			ans[idx] = re
		}
	}

	return ans, nil
}

func packageData(p string) (string, string) {
	cat := ""
	name := ""
	if strings.Contains(p, "/") {
		packagedata := strings.Split(p, "/")
		cat = packagedata[0]
		name = packagedata[1]
	} else {
		name = p
	}
	return cat, name
}
func ParsePackageStr(p string) (*pkg.DefaultPackage, error) {

	if !(strings.HasPrefix(p, "=") || strings.HasPrefix(p, ">") ||
		strings.HasPrefix(p, "<")) {
		ver := ">=0"
		cat := ""
		name := ""

		if strings.Contains(p, "@") {
			packageinfo := strings.Split(p, "@")
			ver = packageinfo[1]
			cat, name = packageData(packageinfo[0])
		} else {
			cat, name = packageData(p)
		}

		return &pkg.DefaultPackage{
			Name:     name,
			Category: cat,
			Version:  ver,
			Uri:      make([]string, 0),
		}, nil
	}

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
			version.PkgSelectorConditionFromInt(gp.Condition.Int()).String(),
			gp.Version,
			gp.VersionSuffix,
			gp.VersionBuild,
		)
	} else {
		pkgVersion = fmt.Sprintf("%s%s%s",
			version.PkgSelectorConditionFromInt(gp.Condition.Int()).String(),
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

func CheckErr(err error) {
	if err != nil {
		Fatal(err)
	}
}
