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
	"github.com/crillab/gophersat/bf"
	version "github.com/hashicorp/go-version"

	"github.com/jinzhu/copier"
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

	GetName() string
	GetVersion() string
	RequiresContains(Package) bool
}

// DefaultPackage represent a standard package definition
type DefaultPackage struct {
	Name             string
	Version          string
	UseFlags         []string
	State            State
	PackageRequires  []*DefaultPackage
	PackageConflicts []*DefaultPackage
	IsSet            bool
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
	return p.Name
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
