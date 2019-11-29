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
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	//	. "github.com/mudler/luet/pkg/logger"

	"github.com/crillab/gophersat/bf"
	version "github.com/hashicorp/go-version"
	"github.com/jinzhu/copier"

	"github.com/ghodss/yaml"
)

// Package is a package interface (TBD)
// FIXME: Currently some of the methods are returning DefaultPackages due to JSON serialization of the package
type Package interface {
	Encode(PackageDatabase) (string, error)

	BuildFormula(PackageDatabase, PackageDatabase) ([]bf.Formula, error)
	IsFlagged(bool) Package
	Flagged() bool
	GetFingerPrint() string
	Requires([]*DefaultPackage) Package
	Conflicts([]*DefaultPackage) Package
	Revdeps(world *[]Package) []Package

	GetRequires() []*DefaultPackage
	GetConflicts() []*DefaultPackage
	Expand(*[]Package) ([]Package, error)
	SetCategory(string)

	GetName() string
	GetCategory() string

	GetVersion() string
	RequiresContains(PackageDatabase, Package) (bool, error)
	Matches(m Package) bool

	AddUse(use string)
	RemoveUse(use string)
	GetUses() []string

	Yaml() ([]byte, error)
	Explain()

	SetPath(string)
	GetPath() string
	Rel(string) string
}

type Tree interface {
	GetPackageSet() PackageDatabase
	Prelude() string // A tree might have a prelude to be able to consume a tree
	SetPackageSet(s PackageDatabase)
	World() ([]Package, error)
	FindPackage(Package) (Package, error)
}

// >> Unmarshallers
// DefaultPackageFromYaml decodes a package from yaml bytes
func DefaultPackageFromYaml(yml []byte) (DefaultPackage, error) {

	var unescaped DefaultPackage
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return DefaultPackage{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return DefaultPackage{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return DefaultPackage{}, err
	}
	return unescaped, nil
}

// Major and minor gets escaped when marshalling in JSON, making compiler fails recognizing selectors for expansion
func (t *DefaultPackage) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// DefaultPackage represent a standard package definition
type DefaultPackage struct {
	ID               int      `json:"-" storm:"id,increment"` // primary key with auto increment
	Name             string   `json:"name"`                   // Affects YAML field names too.
	Version          string   `json:"version"`                // Affects YAML field names too.
	Category         string   `json:"category"`               // Affects YAML field names too.
	UseFlags         []string `json:"use_flags"`              // Affects YAML field names too.
	State            State
	PackageRequires  []*DefaultPackage `json:"requires"`  // Affects YAML field names too.
	PackageConflicts []*DefaultPackage `json:"conflicts"` // Affects YAML field names too.
	IsSet            bool              `json:"set"`       // Affects YAML field names too.

	// TODO: Annotations?

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty"`
}

// State represent the package state
type State string

// NewPackage returns a new package
func NewPackage(name, version string, requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{Name: name, Version: version, PackageRequires: requires, PackageConflicts: conflicts}
}

func (p *DefaultPackage) String() string {
	b, err := p.JSON()
	if err != nil {
		return fmt.Sprintf("{ id: \"%d\", name: \"%s\" }", p.ID, p.Name)
	}
	return fmt.Sprintf("%s", string(b))
}

// GetFingerPrint returns a UUID of the package.
// FIXME: this needs to be unique, now just name is generalized
func (p *DefaultPackage) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}

// GetPath returns the path where the definition file was found
func (p *DefaultPackage) GetPath() string {
	return p.Path
}

func (p *DefaultPackage) Rel(s string) string {
	return filepath.Join(p.GetPath(), s)
}

func (p *DefaultPackage) SetPath(s string) {
	p.Path = s
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
func (p *DefaultPackage) Encode(db PackageDatabase) (string, error) {
	return db.CreatePackage(p)
}

func (p *DefaultPackage) Yaml() ([]byte, error) {
	j, err := p.JSON()
	if err != nil {

		return []byte{}, err
	}
	y, err := yaml.JSONToYAML(j)
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

func (p *DefaultPackage) Matches(m Package) bool {
	if p.GetFingerPrint() == m.GetFingerPrint() {
		return true
	}
	return false
}

func (p *DefaultPackage) Expand(world *[]Package) ([]Package, error) {

	var versionsInWorld []Package
	for _, w := range *world {
		if w.GetName() != p.GetName() || w.GetCategory() != p.GetCategory() {
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

func (p *DefaultPackage) Revdeps(world *[]Package) []Package {
	var versionsInWorld []Package
	for _, w := range *world {
		if w.Matches(p) {
			continue
		}
		for _, re := range w.GetRequires() {
			if re.Matches(p) {
				versionsInWorld = append(versionsInWorld, w)
				versionsInWorld = append(versionsInWorld, w.Revdeps(world)...)
			}
		}
	}

	return versionsInWorld
}

func DecodePackage(ID string, db PackageDatabase) (Package, error) {
	return db.GetPackage(ID)
}

func (pack *DefaultPackage) RequiresContains(definitiondb PackageDatabase, s Package) (bool, error) {
	p, err := definitiondb.FindPackage(pack)
	if err != nil {
		p = pack //relax things
		//return false, errors.Wrap(err, "Package not found in definition db")
	}

	w := definitiondb.World()
	for _, re := range p.GetRequires() {
		if re.Matches(s) {
			return true, nil
		}

		packages, _ := re.Expand(&w)
		for _, pa := range packages {
			if pa.Matches(s) {
				return true, nil
			}
		}
		if contains, err := re.RequiresContains(definitiondb, s); err == nil && contains {
			return true, nil
		}
	}

	return false, nil
}

func Best(set []Package) Package {
	var versionsMap map[string]Package = make(map[string]Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		versionsRaw = append(versionsRaw, p.GetVersion())
		versionsMap[p.GetVersion()] = p
	}

	versions := make([]*version.Version, len(versionsRaw))
	for i, raw := range versionsRaw {
		v, _ := version.NewVersion(raw)
		versions[i] = v
	}

	// After this, the versions are properly sorted
	sort.Sort(version.Collection(versions))

	return versionsMap[versions[len(versions)-1].Original()]
}

func (pack *DefaultPackage) BuildFormula(definitiondb PackageDatabase, db PackageDatabase) ([]bf.Formula, error) {
	// TODO: Expansion needs to go here - and so we ditch Resolvedeps()
	p, err := definitiondb.FindPackage(pack)
	if err != nil {
		p = pack // Relax failures and trust the def
	}
	encodedA, err := p.Encode(db)
	if err != nil {
		return nil, err
	}

	A := bf.Var(encodedA)

	var formulas []bf.Formula
	w := definitiondb.World() // FIXME: this is heavy
	for _, requiredDef := range p.GetRequires() {
		required, err := definitiondb.FindPackage(requiredDef)
		if err != nil {
			//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
			packages, err := requiredDef.Expand(&w)
			//	Info("Expanded", packages, err)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {
				required = Best(packages)

			}
			//required = &DefaultPackage{Name: "test"}
		}

		encodedB, err := required.Encode(db)
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A), B))

		f, err := required.BuildFormula(definitiondb, db)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)

	}

	for _, requiredDef := range p.GetConflicts() {
		required, err := definitiondb.FindPackage(requiredDef)
		if err != nil {
			packages, err := requiredDef.Expand(&w)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {
				for _, p := range packages {
					encodedB, err := p.Encode(db)
					if err != nil {
						return nil, err
					}
					B := bf.Var(encodedB)
					formulas = append(formulas, bf.Or(bf.Not(A),
						bf.Not(B)))

					f, err := p.BuildFormula(definitiondb, db)
					if err != nil {
						return nil, err
					}
					formulas = append(formulas, f...)
				}
			}
		}

		//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
		encodedB, err := required.Encode(db)
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A),
			bf.Not(B)))

		f, err := required.BuildFormula(definitiondb, db)
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
