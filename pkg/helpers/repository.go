// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/config"
)

func GetRepoDatabaseDirPath(name string) string {
	dbpath := filepath.Join(config.LuetCfg.GetSystem().Rootfs, config.LuetCfg.GetSystem().DatabasePath)
	dbpath = filepath.Join(dbpath, "repos/"+name)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return dbpath
}

func GetSystemRepoDatabaseDirPath() string {
	dbpath := filepath.Join(config.LuetCfg.GetSystem().Rootfs,
		config.LuetCfg.GetSystem().DatabasePath)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return dbpath
}

func GetSystemPkgsCacheDirPath() (ans string) {
	var cachepath string
	if config.LuetCfg.GetSystem().PkgsCachePath != "" {
		cachepath = config.LuetCfg.GetSystem().PkgsCachePath
	} else {
		// Create dynamic cache for test suites
		cachepath, _ = ioutil.TempDir(os.TempDir(), "cachepkgs")
	}

	if filepath.IsAbs(cachepath) {
		ans = cachepath
	} else {
		ans = filepath.Join(GetSystemRepoDatabaseDirPath(), cachepath)
	}

	return
}
