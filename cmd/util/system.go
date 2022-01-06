// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package util

import (
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
)

func SystemDB(c *types.LuetConfig) types.PackageDatabase {
	switch c.System.DatabaseEngine {
	case "boltdb":
		return pkg.NewBoltDatabase(
			filepath.Join(c.System.DatabasePath, "luet.db"))
	default:
		return pkg.NewInMemoryDatabase(true)
	}
}
