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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	. "github.com/mudler/luet/pkg/logger"
	tree "github.com/mudler/luet/pkg/tree"

	pkg "github.com/mudler/luet/pkg/package"
)

type MemoryDB int

const (
	InMemory MemoryDB = iota
	BoltDB   MemoryDB = iota
)

func NewGentooBuilder(e EbuildParser, concurrency int, db MemoryDB) tree.Parser {
	return &GentooBuilder{EbuildParser: e, Concurrency: concurrency}
}

type GentooBuilder struct {
	EbuildParser EbuildParser
	Concurrency  int
	DBType       MemoryDB
}

type EbuildParser interface {
	ScanEbuild(string) ([]pkg.Package, error)
}

func (gb *GentooBuilder) scanEbuild(path string, db pkg.PackageDatabase) error {
	defer func() {
		if r := recover(); r != nil {
			Error(r)
		}
	}()
	pkgs, err := gb.EbuildParser.ScanEbuild(path)
	if err != nil {
		return err
	}
	for _, p := range pkgs {
		_, err := db.FindPackage(p)
		if err != nil {
			_, err := db.CreatePackage(p)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (gb *GentooBuilder) worker(i int, wg *sync.WaitGroup, s <-chan string, db pkg.PackageDatabase) {
	defer wg.Done()

	for path := range s {
		Info("#"+strconv.Itoa(i), "parsing", path)
		err := gb.scanEbuild(path, db)
		if err != nil {
			Error(path, ":", err.Error())
		}
	}

}

func (gb *GentooBuilder) Generate(dir string) (pkg.PackageDatabase, error) {

	var toScan = make(chan string)
	Spinner(27)
	defer SpinnerStop()
	var db pkg.PackageDatabase
	// Support for
	switch gb.DBType {
	case InMemory:
		db = pkg.NewInMemoryDatabase(false)
	case BoltDB:
		tmpfile, err := ioutil.TempFile("", "boltdb")
		if err != nil {
			return nil, err
		}
		db = pkg.NewBoltDatabase(tmpfile.Name())
	default:
		db = pkg.NewInMemoryDatabase(false)
	}

	Debug("Concurrency", gb.Concurrency)
	// the waitgroup will allow us to wait for all the goroutines to finish at the end
	var wg = new(sync.WaitGroup)
	for i := 0; i < gb.Concurrency; i++ {
		wg.Add(1)
		go gb.worker(i, wg, toScan, db)
	}

	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), "ebuild") {
			toScan <- path
		}
		return nil
	})

	close(toScan)
	wg.Wait()
	if err != nil {
		return db, err
	}

	return db, nil
}
