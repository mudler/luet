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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/crillab/gophersat/bf"

	"github.com/jinzhu/copier"
)

type Package interface {
	Encode() (string, error)
	SetState(state State) Package
	BuildFormula() ([]bf.Formula, error)
	IsFlagged(bool) Package
	Flagged() bool

	Requires([]*DefaultPackage) Package
	Conflicts([]*DefaultPackage) Package
}

type DefaultPackage struct {
	Name             string
	Version          string
	UseFlags         []string
	State            State
	PackageRequires  []*DefaultPackage
	PackageConflicts []*DefaultPackage
	IsSet            bool
}

type PackageUse []string
type State string

func NewPackage(name, version string, requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{Name: name, Version: version, PackageRequires: requires, PackageConflicts: conflicts}
}

func (p *DefaultPackage) AddUse(use string) {
	for _, v := range p.UseFlags {
		if v == use {
			return
		}
	}
	p.UseFlags = append(p.UseFlags, use)
}

func (p *DefaultPackage) RemoveUse(use string) {

	for i := len(p.UseFlags) - 1; i >= 0; i-- {
		if p.UseFlags[i] == use {
			p.UseFlags = append(p.UseFlags[:i], p.UseFlags[i+1:]...)
		}
	}

}

func (p *DefaultPackage) Encode() (string, error) {
	res, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	enc := base64.StdEncoding.EncodeToString(res)
	crc32q := crc32.MakeTable(0xD5828281)
	ID := fmt.Sprintf("%08x\n", crc32.Checksum([]byte(enc), crc32q))
	Database[ID] = base64.StdEncoding.EncodeToString(res)
	return ID, nil
}

func (p *DefaultPackage) WithState(state State) Package {
	return p.Clone().SetState(state)
}
func (p *DefaultPackage) IsFlagged(b bool) Package {
	p.IsSet = b
	return p
}
func (p *DefaultPackage) Flagged() bool {
	return p.IsSet
}

func (p *DefaultPackage) SetState(state State) Package {
	p.State = state
	return p
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

func DecodePackage(ID string) (Package, error) {

	pa, ok := Database[ID]
	if !ok {
		return nil, errors.New("No package found with that id")
	}

	enc, err := base64.StdEncoding.DecodeString(pa)
	if err != nil {
		return nil, err
	}
	p := &DefaultPackage{}

	if err := json.Unmarshal(enc, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *DefaultPackage) BuildFormula() ([]bf.Formula, error) {
	encodedA, err := p.IsFlagged(true).Encode()
	if err != nil {
		return nil, err
	}

	A := bf.Var(encodedA)

	var formulas []bf.Formula

	for _, required := range p.PackageRequires {
		encodedB, err := required.IsFlagged(true).Encode()
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
		encodedB, err := required.IsFlagged(true).Encode()
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
