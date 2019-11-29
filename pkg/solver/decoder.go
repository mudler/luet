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
	toposort "github.com/philopon/go-toposort"
	"github.com/stevenle/topsort"
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
	if a.Package.Flagged() {
		msg = "installed"
	} else {
		msg = "not installed"
	}
	return fmt.Sprintf("%s/%s %s %s: %t", a.Package.GetCategory(), a.Package.GetName(), a.Package.GetVersion(), msg, a.Value)
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

func (assertions PackagesAssertions) Order(fingerprint string) PackagesAssertions {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}
	fingerprints := []string{}

	tmpMap := map[string]PackageAssert{}
	graph := topsort.NewGraph()

	for _, a := range assertions {
		graph.AddNode(a.Package.GetFingerPrint())
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
	//graph := toposort.NewGraph(len(unorderedAssertions))
	//	graph.AddNodes(fingerprints...)
	for _, a := range unorderedAssertions {
		for _, req := range a.Package.GetRequires() {
			graph.AddEdge(a.Package.GetFingerPrint(), req.GetFingerPrint())
		}
	}
	result, err := graph.TopSort(fingerprint)
	if err != nil {
		panic(err)
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

func (assertions PackagesAssertions) AssertionHash() string {
	var fingerprint string
	for _, assertion := range assertions { // Note: Always order them first!
		if assertion.Value { // Tke into account only dependencies installed (get fingerprint of subgraph)
			fingerprint += assertion.ToString() + "\n"
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
