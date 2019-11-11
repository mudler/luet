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

	pkg "github.com/mudler/luet/pkg/package"
	toposort "github.com/philopon/go-toposort"
)

type PackagesAssertions []PackageAssert

// PackageAssert represent a package assertion.
// It is composed of a Package and a Value which is indicating the absence or not
// of the associated package state.
type PackageAssert struct {
	Package pkg.Package
	Value   bool
}

// DecodeModel decodes a model from the SAT solver to package assertions (PackageAssert)
func DecodeModel(model map[string]bool) (PackagesAssertions, error) {
	ass := make(PackagesAssertions, 0)
	for k, v := range model {
		a, err := pkg.DecodePackage(k)
		if err != nil {
			return nil, err

		}
		ass = append(ass, PackageAssert{Package: a, Value: v})
	}
	return ass, nil
}

func (a *PackageAssert) Explain() {
	fmt.Println(a.ToString())
	a.Package.Explain()
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

func (assertions PackagesAssertions) Order() PackagesAssertions {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}
	fingerprints := []string{}

	tmpMap := map[string]PackageAssert{}

	for _, a := range assertions {
		if a.Package.Flagged() {
			unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered
			fingerprints = append(fingerprints, a.Package.GetFingerPrint())
			tmpMap[a.Package.GetFingerPrint()] = a
		} else {
			orderedAssertions = append(orderedAssertions, a) // Keep last the ones which are not meant to be installed
		}
	}

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
		panic("cycle detected")
	}

	for _, res := range result {
		a, ok := tmpMap[res]
		if !ok {
			panic("Sort order - this shouldn't happen")
		}
		orderedAssertions = append([]PackageAssert{a}, orderedAssertions...) // push upfront
	}

	return orderedAssertions
}

func (assertions PackagesAssertions) Explain() string {
	var fingerprint string
	for _, assertion := range assertions.Order() { // Always order them
		fingerprint += assertion.ToString() + "\n"
	}
	return fingerprint
}

func (assertions PackagesAssertions) AssertionHash() string {
	var fingerprint string
	for _, assertion := range assertions.Order() { // Always order them
		if assertion.Value && assertion.Package.Flagged() { // Tke into account only dependencies installed (get fingerprint of subgraph)
			fingerprint += assertion.ToString() + "\n"
		}
	}
	hash := sha256.Sum256([]byte(fingerprint))
	return fmt.Sprintf("%x", hash)
}

func (assertions PackagesAssertions) Drop(p pkg.Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if a.Package.GetFingerPrint() != p.GetFingerPrint() {
			ass = append(ass, a)
		}
	}
	return ass
}
