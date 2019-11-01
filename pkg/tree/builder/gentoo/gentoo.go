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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tree "github.com/mudler/luet/pkg/tree"

	pkg "github.com/mudler/luet/pkg/package"
)

func NewGentooBuilder(e EbuildParser, concurrency int) tree.Parser {
	return &GentooBuilder{EbuildParser: e, Concurrency: concurrency}
}

type GentooBuilder struct {
	EbuildParser EbuildParser
	Concurrency  int
}

type GentooTree struct {
	*tree.DefaultTree
}

type EbuildParser interface {
	ScanEbuild(string, pkg.Tree) ([]pkg.Package, error)
}

func (gt *GentooTree) Prelude() string {
	return "/usr/portage/"
}

func (gb *GentooBuilder) scanEbuild(path string, t pkg.Tree) error {

	fmt.Println("Scanning ", path)
	pkgs, err := gb.EbuildParser.ScanEbuild(path, t)
	if err != nil {
		return err
	}
	for _, p := range pkgs {
		_, err := t.GetPackageSet().FindPackage(p)
		if err != nil {
			_, err := t.GetPackageSet().CreatePackage(p)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func (gb *GentooBuilder) worker(i int, wg *sync.WaitGroup, s chan string, t pkg.Tree) error {
	defer wg.Done()

	for path := range s {
		err := gb.scanEbuild(path, t)
		if err != nil {
			fmt.Println("Error scanning ebuild: " + path)
		}
	}

	return nil
}

func (gb *GentooBuilder) Generate(dir string) (pkg.Tree, error) {
	tmpfile, err := ioutil.TempFile("", "boltdb")
	if err != nil {
		return nil, err
	}
	var toScan = make(chan string)
	tree := &GentooTree{DefaultTree: &tree.DefaultTree{Packages: pkg.NewBoltDatabase(tmpfile.Name())}}

	// the waitgroup will allow us to wait for all the goroutines to finish at the end
	var wg = new(sync.WaitGroup)
	for i := 0; i < gb.Concurrency; i++ {
		wg.Add(1)
		go gb.worker(i, wg, toScan, tree)
	}

	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
		return tree, err
	}

	close(toScan)
	wg.Wait()

	return tree, tree.ResolveDeps()
}
