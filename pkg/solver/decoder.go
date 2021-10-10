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

package solver

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"unicode"

	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/topsort"
	toposort "github.com/philopon/go-toposort"
	"github.com/pkg/errors"
)

type PackagesAssertions []PackageAssert

type PackageHash struct {
	BuildHash   string
	PackageHash string
}

// PackageAssert represent a package assertion.
// It is composed of a Package and a Value which is indicating the absence or not
// of the associated package state.
type PackageAssert struct {
	Package *pkg.DefaultPackage
	Value   bool
	Hash    PackageHash
}

// DecodeModel decodes a model from the SAT solver to package assertions (PackageAssert)
func DecodeModel(model map[string]bool, db pkg.PackageDatabase) (PackagesAssertions, error) {
	ass := make(PackagesAssertions, 0)
	for k, v := range model {
		a, err := pkg.DecodePackage(k, db)
		if err != nil {
			return nil, err

		}
		ass = append(ass, PackageAssert{Package: a.(*pkg.DefaultPackage), Value: v})
	}
	return ass, nil
}

func (a *PackageAssert) Explain() {
	fmt.Println(a.ToString())
	a.Package.Explain()
}

func (a *PackageAssert) String() string {
	return a.ToString()
}

func (a *PackageAssert) ToString() string {
	var msg string
	if a.Value {
		msg = "installed"
	} else {
		msg = "not installed"
	}
	return fmt.Sprintf("%s/%s %s %s", a.Package.GetCategory(), a.Package.GetName(), a.Package.GetVersion(), msg)
}

func (assertions PackagesAssertions) EnsureOrder() PackagesAssertions {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}
	fingerprints := []string{}

	tmpMap := map[string]PackageAssert{}

	for _, a := range assertions {
		tmpMap[a.Package.GetFingerPrint()] = a
		fingerprints = append(fingerprints, a.Package.GetFingerPrint())
		unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered

		if a.Value {
			unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered
		} else {
			orderedAssertions = append(orderedAssertions, a) // Keep last the ones which are not meant to be installed
		}
	}

	sort.Sort(unorderedAssertions)

	// Build a topological graph
	graph := toposort.NewGraph(len(unorderedAssertions))
	graph.AddNodes(fingerprints...)
	for _, a := range unorderedAssertions {
		for _, req := range a.Package.GetRequires() {
			graph.AddEdge(a.Package.GetFingerPrint(), req.GetFingerPrint())
		}
	}
	result, ok := graph.Toposort()
	if !ok {
		panic("Cycle found")
	}
	for _, res := range result {
		a, ok := tmpMap[res]
		if !ok {
			panic("fail")
			//	continue
		}
		orderedAssertions = append(orderedAssertions, a)
		//	orderedAssertions = append(PackagesAssertions{a}, orderedAssertions...) // push upfront
	}
	//helpers.ReverseAny(orderedAssertions)
	return orderedAssertions
}

func (assertions PackagesAssertions) SearchByName(f string) *PackageAssert {
	for _, a := range assertions {
		if a.Value {
			if a.Package.GetPackageName() == f {
				return &a
			}
		}
	}

	return nil
}
func (assertions PackagesAssertions) Search(f string) *PackageAssert {
	for _, a := range assertions {
		if a.Value {
			if a.Package.GetFingerPrint() == f {
				return &a
			}
		}
	}

	return nil
}

func (assertions PackagesAssertions) ToDB() pkg.PackageDatabase {
	db := pkg.NewInMemoryDatabase(false)
	for _, a := range assertions {
		if a.Value {
			db.CreatePackage(a.Package)
		}
	}

	return db
}

func (assertions PackagesAssertions) Order(definitiondb pkg.PackageDatabase, fingerprint string) (PackagesAssertions, error) {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}

	tmpMap := map[string]PackageAssert{}
	graph := topsort.NewGraph()
	for _, a := range assertions {
		graph.AddNode(a.Package.GetFingerPrint())
		tmpMap[a.Package.GetFingerPrint()] = a
		unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered
	}

	sort.Sort(unorderedAssertions)
	// Build a topological graph
	for _, a := range unorderedAssertions {
		currentPkg := a.Package
		added := map[string]interface{}{}
	REQUIRES:
		for _, requiredDef := range currentPkg.GetRequires() {
			if def, err := definitiondb.FindPackage(requiredDef); err == nil { // Provides: Get a chance of being override here
				requiredDef = def.(*pkg.DefaultPackage)
			}

			// We cannot search for fingerprint, as we could have selector in versions.
			// We know that the assertions are unique for packages, so look for a package with such name in the assertions
			req := assertions.SearchByName(requiredDef.GetPackageName())
			if req != nil {
				requiredDef = req.Package
			}
			if _, ok := added[requiredDef.GetFingerPrint()]; ok {
				continue REQUIRES
			}
			// Expand also here, as we need to order them (or instead the solver should give back the dep correctly?)
			graph.AddEdge(currentPkg.GetFingerPrint(), requiredDef.GetFingerPrint())
			added[requiredDef.GetFingerPrint()] = true
		}
	}
	result, err := graph.TopSort(fingerprint)
	if err != nil {
		return nil, errors.Wrap(err, "fail on sorting "+fingerprint)
	}
	for _, res := range result {
		a, ok := tmpMap[res]
		if !ok {
			//return nil, errors.New("fail looking for " + res)
			// Since now we don't return the entire world as part of assertions
			// if we don't find any reference must be because fingerprint we are analyzing (which is the one we are ordering against)
			// is not part of the assertions, thus we can omit it from the result
			continue
		}
		orderedAssertions = append(orderedAssertions, a)
		//	orderedAssertions = append(PackagesAssertions{a}, orderedAssertions...) // push upfront
	}
	//helpers.ReverseAny(orderedAssertions)
	return orderedAssertions, nil
}

func (assertions PackagesAssertions) Explain() string {
	var fingerprint string
	for _, assertion := range assertions { // Always order them
		fingerprint += assertion.ToString() + "\n"
	}
	return fingerprint
}

func (a PackagesAssertions) Len() int      { return len(a) }
func (a PackagesAssertions) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PackagesAssertions) Less(i, j int) bool {

	iRunes := []rune(a[i].Package.GetName())
	jRunes := []rune(a[j].Package.GetName())

	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}

	return false

}

func (a PackagesAssertions) TrueLen() int {
	count := 0
	for _, ass := range a {
		if ass.Value {
			count++
		}
	}

	return count
}

// HashFrom computes the assertion hash From a given package. It drops it from the assertions
// and checks it's not the only one. if it's unique it marks it specially - so the hash
// which is generated is unique for the selected package
func (assertions PackagesAssertions) HashFrom(p pkg.Package) string {
	return assertions.SaltedHashFrom(p, map[string]string{})
}

func (assertions PackagesAssertions) AssertionHash() string {
	return assertions.SaltedAssertionHash(map[string]string{})
}

func (assertions PackagesAssertions) SaltedHashFrom(p pkg.Package, salts map[string]string) string {
	var assertionhash string

	// When we don't have any solution to hash for, we need to generate an UUID by ourselves
	latestsolution := assertions.Drop(p)
	if latestsolution.TrueLen() == 0 {
		// Preserve the hash if supplied of marked packages
		marked := p.Mark()
		if markedHash, exists := salts[p.GetFingerPrint()]; exists {
			salts[marked.GetFingerPrint()] = markedHash
		}
		assertionhash = assertions.Mark(p).SaltedAssertionHash(salts)
	} else {
		assertionhash = latestsolution.SaltedAssertionHash(salts)
	}
	return assertionhash
}

func (assertions PackagesAssertions) SaltedAssertionHash(salts map[string]string) string {
	var fingerprint string
	for _, assertion := range assertions { // Note: Always order them first!
		if assertion.Value { // Tke into account only dependencies installed (get fingerprint of subgraph)
			salt, exists := salts[assertion.Package.GetFingerPrint()]
			if exists {
				fingerprint += assertion.ToString() + salt + "\n"

			} else {
				fingerprint += assertion.ToString() + "\n"
			}
		}
	}
	hash := sha256.Sum256([]byte(fingerprint))
	return fmt.Sprintf("%x", hash)
}

func (assertions PackagesAssertions) Drop(p pkg.Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if !a.Package.Matches(p) {
			ass = append(ass, a)
		}
	}
	return ass
}

// Cut returns an assertion list of installed (filter by Value) "cutted" until the package is found (included)
func (assertions PackagesAssertions) Cut(p pkg.Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if a.Value {
			ass = append(ass, a)
			if a.Package.Matches(p) {
				break
			}
		}
	}
	return ass
}

// Mark returns a new assertion with the package marked
func (assertions PackagesAssertions) Mark(p pkg.Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if a.Package.Matches(p) {
			marked := a.Package.Mark()
			a = PackageAssert{Package: marked.(*pkg.DefaultPackage), Value: a.Value, Hash: a.Hash}
		}
		ass = append(ass, a)
	}
	return ass
}
