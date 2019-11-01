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

package pkg

import (
	"fmt"

	"github.com/crillab/gophersat/bf"
	version "github.com/hashicorp/go-version"
	"github.com/jinzhu/copier"

	"github.com/ghodss/yaml"
)

// Package is a package interface (TBD)
// FIXME: Currently some of the methods are returning DefaultPackages due to JSON serialization of the package
type Package interface {
	Encode() (string, error)

	BuildFormula() ([]bf.Formula, error)
	IsFlagged(bool) Package
	Flagged() bool
	GetFingerPrint() string
	Requires([]*DefaultPackage) Package
	Conflicts([]*DefaultPackage) Package

	GetRequires() []*DefaultPackage
	GetConflicts() []*DefaultPackage
	Expand([]Package) ([]Package, error)
	SetCategory(string)

	GetName() string
	GetCategory() string

	GetVersion() string
	RequiresContains(Package) bool

	AddUse(use string)
	RemoveUse(use string)
	GetUses() []string

	Yaml() ([]byte, error)
	Explain()
}

type PackageSet interface {
	GetPackages() []string //Ids
	CreatePackage(pkg Package) (string, error)
	GetPackage(ID string) (Package, error)
	Clean() error
	FindPackage(Package) (Package, error)
	UpdatePackage(p Package) error
	GetAllPackages(packages chan Package) error
}

type Tree interface {
	GetPackageSet() PackageSet
	Prelude() string // A tree might have a prelude to be able to consume a tree
	SetPackageSet(s PackageSet)
	World() ([]Package, error)
	FindPackage(Package) (Package, error)
}

// >> Unmarshallers
// DefaultPackageFromYaml decodes a package from yaml bytes
func DefaultPackageFromYaml(source []byte) (DefaultPackage, error) {
	var pkg DefaultPackage
	err := yaml.Unmarshal(source, &pkg)
	if err != nil {
		return pkg, err
	}
	return pkg, nil
}

// DefaultPackage represent a standard package definition
type DefaultPackage struct {
	ID               int      `storm:"id,increment"` // primary key with auto increment
	Name             string   `json:"name"`          // Affects YAML field names too.
	Version          string   `json:"version"`       // Affects YAML field names too.
	Category         string   `json:"category"`      // Affects YAML field names too.
	UseFlags         []string `json:"use_flags"`     // Affects YAML field names too.
	State            State
	PackageRequires  []*DefaultPackage `json:"requires"`  // Affects YAML field names too.
	PackageConflicts []*DefaultPackage `json:"conflicts"` // Affects YAML field names too.
	IsSet            bool              `json:"set"`       // Affects YAML field names too.
}

// State represent the package state
type State string

// NewPackage returns a new package
func NewPackage(name, version string, requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{Name: name, Version: version, PackageRequires: requires, PackageConflicts: conflicts}
}

// GetFingerPrint returns a UUID of the package.
// FIXME: this needs to be unique, now just name is generalized
func (p *DefaultPackage) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}

// AddUse adds a use to a package
func (p *DefaultPackage) AddUse(use string) {
	for _, v := range p.UseFlags {
		if v == use {
			return
		}
	}
	p.UseFlags = append(p.UseFlags, use)
}

// RemoveUse removes a use to a package
func (p *DefaultPackage) RemoveUse(use string) {

	for i := len(p.UseFlags) - 1; i >= 0; i-- {
		if p.UseFlags[i] == use {
			p.UseFlags = append(p.UseFlags[:i], p.UseFlags[i+1:]...)
		}
	}

}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (p *DefaultPackage) Encode() (string, error) {
	return NewInMemoryDatabase().CreatePackage(p)
}

func (p *DefaultPackage) Yaml() ([]byte, error) {
	y, err := yaml.Marshal(p)
	if err != nil {

		return []byte{}, err
	}
	return y, nil
}

func (p *DefaultPackage) IsFlagged(b bool) Package {
	p.IsSet = b
	return p
}

func (p *DefaultPackage) Flagged() bool {
	return p.IsSet
}

func (p *DefaultPackage) GetName() string {
	return p.Name
}

func (p *DefaultPackage) GetVersion() string {
	return p.Version
}

func (p *DefaultPackage) GetCategory() string {
	return p.Category
}

func (p *DefaultPackage) SetCategory(s string) {
	p.Category = s
}
func (p *DefaultPackage) GetUses() []string {
	return p.UseFlags
}

func (p *DefaultPackage) GetRequires() []*DefaultPackage {
	return p.PackageRequires
}
func (p *DefaultPackage) GetConflicts() []*DefaultPackage {
	return p.PackageConflicts
}
func (p *DefaultPackage) Requires(req []*DefaultPackage) Package {
	p.PackageRequires = req
	return p
}
func (p *DefaultPackage) Conflicts(req []*DefaultPackage) Package {
	p.PackageConflicts = req
	return p
}
func (p *DefaultPackage) Clone() Package {
	new := &DefaultPackage{}
	copier.Copy(&new, &p)
	return new
}

func (p *DefaultPackage) Expand(world []Package) ([]Package, error) {

	var versionsInWorld []Package
	for _, w := range world {
		if w.GetName() != p.GetName() {
			continue
		}

		v, err := version.NewVersion(w.GetVersion())
		if err != nil {
			return nil, err
		}
		constraints, err := version.NewConstraint(p.GetVersion())
		if err != nil {
			return nil, err
		}
		if constraints.Check(v) {
			versionsInWorld = append(versionsInWorld, w)
		}
	}

	return versionsInWorld, nil
}

func DecodePackage(ID string) (Package, error) {
	return NewInMemoryDatabase().GetPackage(ID)
}

func NormalizeFlagged(p Package) {
	for _, r := range p.GetRequires() {
		r.IsFlagged(true)
		NormalizeFlagged(r)
	}
	for _, r := range p.GetConflicts() {
		r.IsFlagged(true)
		NormalizeFlagged(r)
	}
}

func (p *DefaultPackage) RequiresContains(s Package) bool {
	for _, re := range p.GetRequires() {
		if re.GetFingerPrint() == s.GetFingerPrint() {
			return true
		}

		if re.RequiresContains(s) {
			return true
		}
	}

	return false
}

func (p *DefaultPackage) BuildFormula() ([]bf.Formula, error) {
	encodedA, err := p.IsFlagged(true).Encode()
	if err != nil {
		return nil, err
	}
	NormalizeFlagged(p)

	A := bf.Var(encodedA)

	var formulas []bf.Formula

	for _, required := range p.PackageRequires {
		encodedB, err := required.Encode()
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A), B))

		f, err := required.BuildFormula()
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)

	}

	for _, required := range p.PackageConflicts {
		encodedB, err := required.Encode()
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A),
			bf.Not(B)))

		f, err := required.BuildFormula()
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)
	}
	return formulas, nil
}

func (p *DefaultPackage) Explain() {

	fmt.Println("====================")
	fmt.Println("Name: ", p.GetName())
	fmt.Println("Category: ", p.GetCategory())
	fmt.Println("Version: ", p.GetVersion())
	fmt.Println("Installed: ", p.IsSet)

	for _, req := range p.GetRequires() {
		fmt.Println("\t-> ", req)
	}

	for _, con := range p.GetConflicts() {
		fmt.Println("\t!! ", con)
	}

	fmt.Println("====================")

}
