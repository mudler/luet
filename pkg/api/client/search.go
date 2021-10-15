// Copyright © 2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mudler/luet/pkg/api/client/utils"
)

func TreePackages(treedir string) (searchResult SearchResult, err error) {
	var res []byte
	res, err = utils.RunSHOUT("tree", fmt.Sprintf("luet tree pkglist --tree %s --output json", treedir))
	if err != nil {
		fmt.Println(string(res))
		return
	}
	json.Unmarshal(res, &searchResult)
	return
}

func imageAvailable(image string) bool {
	_, err := crane.Digest(image)
	return err == nil
}

type SearchResult struct {
	Packages []Package
}

type Package struct {
	Name, Category, Version, Path string
}

func (p Package) String() string {
	return fmt.Sprintf("%s/%s@%s", p.Category, p.Name, p.Version)
}

func (p Package) Image(repository string) string {
	return fmt.Sprintf("%s:%s-%s-%s", repository, p.Name, p.Category, strings.ReplaceAll(p.Version, "+", "-"))
}

func (p Package) ImageTag() string {
	// ${name}-${category}-${version//+/-}
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, strings.ReplaceAll(p.Version, "+", "-"))
}

func (p Package) ImageMetadata(repository string) string {
	return fmt.Sprintf("%s.metadata.yaml", p.Image(repository))
}

func (p Package) ImageAvailable(repository string) bool {
	return imageAvailable(p.Image(repository))
}

func (p Package) Equal(pp Package) bool {
	if p.Name == pp.Name && p.Category == pp.Category && p.Version == pp.Version {
		return true
	}
	return false
}

func (p Package) EqualS(s string) bool {
	if s == fmt.Sprintf("%s/%s", p.Category, p.Name) {
		return true
	}
	return false
}

func (p Package) EqualSV(s string) bool {
	if s == fmt.Sprintf("%s/%s@%s", p.Category, p.Name, p.Version) {
		return true
	}
	return false
}

func (p Package) EqualNoV(pp Package) bool {
	if p.Name == pp.Name && p.Category == pp.Category {
		return true
	}
	return false
}

func (s SearchResult) FilterByCategory(cat string) SearchResult {
	new := SearchResult{Packages: []Package{}}

	for _, r := range s.Packages {
		if r.Category == cat {
			new.Packages = append(new.Packages, r)
		}
	}
	return new
}

func (s SearchResult) FilterByName(name string) SearchResult {
	new := SearchResult{Packages: []Package{}}

	for _, r := range s.Packages {
		if !strings.Contains(r.Name, name) {
			new.Packages = append(new.Packages, r)
		}
	}
	return new
}

type Packages []Package

func (p Packages) Exist(pp Package) bool {
	for _, pi := range p {
		if pp.Equal(pi) {
			return true
		}
	}
	return false
}
