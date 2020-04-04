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
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	"github.com/crillab/gophersat/bf"
	"github.com/ghodss/yaml"
	version "github.com/hashicorp/go-version"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

// Package is a package interface (TBD)
// FIXME: Currently some of the methods are returning DefaultPackages due to JSON serialization of the package
type Package interface {
	Encode(PackageDatabase) (string, error)

	BuildFormula(PackageDatabase, PackageDatabase) ([]bf.Formula, error)
	IsFlagged(bool) Package
	Flagged() bool
	GetFingerPrint() string
	GetPackageName() string
	Requires([]*DefaultPackage) Package
	Conflicts([]*DefaultPackage) Package
	Revdeps(PackageDatabase) Packages
	LabelDeps(PackageDatabase, string) Packages

	GetProvides() []*DefaultPackage
	SetProvides([]*DefaultPackage) Package

	GetRequires() []*DefaultPackage
	GetConflicts() []*DefaultPackage
	Expand(PackageDatabase) (Packages, error)
	SetCategory(string)

	GetName() string
	GetCategory() string

	GetVersion() string
	RequiresContains(PackageDatabase, Package) (bool, error)
	Matches(m Package) bool
	BumpBuildVersion() error

	AddUse(use string)
	RemoveUse(use string)
	GetUses() []string

	Yaml() ([]byte, error)
	Explain()

	SetPath(string)
	GetPath() string
	Rel(string) string

	GetDescription() string
	SetDescription(string)

	AddURI(string)
	GetURI() []string

	SetLicense(string)
	GetLicense() string

	AddLabel(string, string)
	GetLabels() map[string]string
	HasLabel(string) bool
	MatchLabel(*regexp.Regexp) bool

	IsSelector() bool
	VersionMatchSelector(string) (bool, error)
	SelectorMatchVersion(string) (bool, error)

	String() string
	HumanReadableString() string
	HashFingerprint() string
}

type Tree interface {
	GetPackageSet() PackageDatabase
	Prelude() string // A tree might have a prelude to be able to consume a tree
	SetPackageSet(s PackageDatabase)
	World() (Packages, error)
	FindPackage(Package) (Package, error)
}

type Packages []Package

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
	ID               int               `storm:"id,increment" json:"id"` // primary key with auto increment
	Name             string            `json:"name"`                    // Affects YAML field names too.
	Version          string            `json:"version"`                 // Affects YAML field names too.
	Category         string            `json:"category"`                // Affects YAML field names too.
	UseFlags         []string          `json:"use_flags,omitempty"`     // Affects YAML field names too.
	State            State             `json:"state,omitempty"`
	PackageRequires  []*DefaultPackage `json:"requires"`           // Affects YAML field names too.
	PackageConflicts []*DefaultPackage `json:"conflicts"`          // Affects YAML field names too.
	IsSet            bool              `json:"set,omitempty"`      // Affects YAML field names too.
	Provides         []*DefaultPackage `json:"provides,omitempty"` // Affects YAML field names too.

	// TODO: Annotations?

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty"`

	Description string   `json:"description,omitempty"`
	Uri         []string `json:"uri,omitempty"`
	License     string   `json:"license,omitempty"`

	Labels map[string]string `json:labels,omitempty`
}

// State represent the package state
type State string

// NewPackage returns a new package
func NewPackage(name, version string, requires []*DefaultPackage, conflicts []*DefaultPackage) *DefaultPackage {
	return &DefaultPackage{
		Name:             name,
		Version:          version,
		PackageRequires:  requires,
		PackageConflicts: conflicts,
		Labels:           make(map[string]string, 0),
	}
}

func (p *DefaultPackage) String() string {
	b, err := p.JSON()
	if err != nil {
		return fmt.Sprintf("{ id: \"%d\", name: \"%s\", version: \"%s\", category: \"%s\"  }", p.ID, p.Name, p.Version, p.Category)
	}
	return fmt.Sprintf("%s", string(b))
}

// GetFingerPrint returns a UUID of the package.
// FIXME: this needs to be unique, now just name is generalized
func (p *DefaultPackage) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}

func (p *DefaultPackage) HashFingerprint() string {
	h := md5.New()
	io.WriteString(h, p.GetFingerPrint())
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *DefaultPackage) HumanReadableString() string {
	return fmt.Sprintf("%s/%s-%s", p.Category, p.Name, p.Version)
}

func FromString(s string) Package {
	var unescaped DefaultPackage

	err := json.Unmarshal([]byte(s), &unescaped)
	if err != nil {
		return &unescaped
	}
	return &unescaped
}

func (p *DefaultPackage) GetPackageName() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Category)
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

func (p *DefaultPackage) IsSelector() bool {
	return strings.ContainsAny(p.GetVersion(), "<>=")
}

func (p *DefaultPackage) HasLabel(label string) bool {
	ans := false
	for k, _ := range p.Labels {
		if k == label {
			ans = true
			break
		}
	}
	return ans
}

func (p *DefaultPackage) MatchLabel(r *regexp.Regexp) bool {
	ans := false
	for k, v := range p.Labels {
		if r.MatchString(k + "=" + v) {
			ans = true
			break
		}
	}
	return ans
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
func (p *DefaultPackage) GetDescription() string {
	return p.Description
}
func (p *DefaultPackage) SetDescription(s string) {
	p.Description = s
}
func (p *DefaultPackage) GetLicense() string {
	return p.License
}
func (p *DefaultPackage) SetLicense(s string) {
	p.License = s
}
func (p *DefaultPackage) AddURI(s string) {
	p.Uri = append(p.Uri, s)
}
func (p *DefaultPackage) GetURI() []string {
	return p.Uri
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
func (p *DefaultPackage) AddLabel(k, v string) {
	p.Labels[k] = v
}
func (p *DefaultPackage) GetLabels() map[string]string {
	return p.Labels
}
func (p *DefaultPackage) GetProvides() []*DefaultPackage {
	return p.Provides
}
func (p *DefaultPackage) SetProvides(req []*DefaultPackage) Package {
	p.Provides = req
	return p
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

func (p *DefaultPackage) Expand(definitiondb PackageDatabase) (Packages, error) {
	var versionsInWorld Packages

	all, err := definitiondb.FindPackages(p)
	if err != nil {
		return nil, err
	}
	for _, w := range all {
		match, err := p.SelectorMatchVersion(w.GetVersion())
		if err != nil {
			return nil, err
		}
		if match {
			versionsInWorld = append(versionsInWorld, w)
		}
	}

	return versionsInWorld, nil
}

func (p *DefaultPackage) Revdeps(definitiondb PackageDatabase) Packages {
	var versionsInWorld Packages
	for _, w := range definitiondb.World() {
		if w.Matches(p) {
			continue
		}
		for _, re := range w.GetRequires() {
			if re.Matches(p) {
				versionsInWorld = append(versionsInWorld, w)
				versionsInWorld = append(versionsInWorld, w.Revdeps(definitiondb)...)
			}
		}
	}

	return versionsInWorld
}

func (p *DefaultPackage) LabelDeps(definitiondb PackageDatabase, labelKey string) Packages {
	var pkgsWithLabelInWorld Packages
	// TODO: check if integrate some index to improve
	// research instead of iterate all list.
	for _, w := range definitiondb.World() {
		if w.HasLabel(labelKey) {
			pkgsWithLabelInWorld = append(pkgsWithLabelInWorld, w)
		}
	}

	return pkgsWithLabelInWorld
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

	for _, re := range p.GetRequires() {
		if re.Matches(s) {
			return true, nil
		}

		packages, _ := re.Expand(definitiondb)
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

func (set Packages) Best() Package {
	var versionsMap map[string]Package = make(map[string]Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		// TODO: This is temporary!.
		sanitizedVersion := strings.ReplaceAll(p.GetVersion(), "_", "-")
		versionsRaw = append(versionsRaw, sanitizedVersion)
		versionsMap[sanitizedVersion] = p
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

	// Do conflict with other packages versions (if A is selected, then conflict with other versions of A)
	packages, _ := definitiondb.FindPackageVersions(p)
	if len(packages) > 0 {
		for _, cp := range packages {
			encodedB, err := cp.Encode(db)
			if err != nil {
				return nil, err
			}
			B := bf.Var(encodedB)
			if !p.Matches(cp) {
				formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.Not(A), bf.Not(B))))
			}
		}
	}

	for _, requiredDef := range p.GetRequires() {
		required, err := definitiondb.FindPackage(requiredDef)
		if err != nil || requiredDef.IsSelector() {
			if err == nil {
				requiredDef = required.(*DefaultPackage)
			}

			packages, err := definitiondb.FindPackages(requiredDef)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {

				var ALO, priorityConstraints, priorityALO []bf.Formula

				// Try to prio best match
				// Force the solver to consider first our candidate (if does exists).
				// Then builds ALO and AMO for the requires.
				c, candidateErr := definitiondb.FindPackageCandidate(requiredDef)
				var C bf.Formula
				if candidateErr == nil {
					// We have a desired candidate, try to look a solution with that included first
					for _, o := range packages {
						encodedB, err := o.Encode(db)
						if err != nil {
							return nil, err
						}
						B := bf.Var(encodedB)
						if !o.Matches(c) {
							priorityConstraints = append(priorityConstraints, bf.Not(B))
							priorityALO = append(priorityALO, B)
						}
					}
					encodedC, err := c.Encode(db)
					if err != nil {
						return nil, err
					}
					C = bf.Var(encodedC)
					// Or the Candidate is true, or all the others might be not true
					// This forces the CDCL sat implementation to look first at a solution with C=true
					formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.Or(C, bf.Or(priorityConstraints...)), bf.Or(bf.Not(C), bf.Or(priorityALO...)))))
				}

				// AMO - At most one
				for _, o := range packages {
					encodedB, err := o.Encode(db)
					if err != nil {
						return nil, err
					}
					B := bf.Var(encodedB)
					ALO = append(ALO, B)
					for _, i := range packages {
						encodedI, err := i.Encode(db)
						if err != nil {
							return nil, err
						}
						I := bf.Var(encodedI)
						if !o.Matches(i) {
							formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.Not(I), bf.Not(B))))
						}
					}
				}
				formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(ALO...))) // ALO - At least one
				continue
			}

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
		if err != nil || requiredDef.IsSelector() {
			if err == nil {
				requiredDef = required.(*DefaultPackage)
			}
			packages, err := definitiondb.FindPackages(requiredDef)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {
				if len(packages) == 1 {
					required = packages[0]
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
					continue
				}
			}
		}

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

func (p *DefaultPackage) BumpBuildVersion() error {
	cat := p.Category
	if cat == "" {
		// Use fake category for parse package
		cat = "app"
	}
	gp, err := gentoo.ParsePackageStr(
		fmt.Sprintf("%s/%s-%s", cat,
			p.Name, p.GetVersion()))
	if err != nil {
		return errors.Wrap(err, "Error on parser version")
	}

	buildPrefix := ""
	buildId := 0

	if gp.VersionBuild != "" {
		// Check if version build is a number
		buildId, err = strconv.Atoi(gp.VersionBuild)
		if err == nil {
			goto end
		}
		// POST: is not only a number

		// TODO: check if there is a better way to handle all use cases.

		r1 := regexp.MustCompile(`^r[0-9]*$`)
		if r1 == nil {
			return errors.New("Error on create regex for -r[0-9]")
		}
		if r1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "r", ""))
			if err == nil {
				buildPrefix = "r"
				goto end
			}
		}

		p1 := regexp.MustCompile(`^p[0-9]*$`)
		if p1 == nil {
			return errors.New("Error on create regex for -p[0-9]")
		}
		if p1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "p", ""))
			if err == nil {
				buildPrefix = "p"
				goto end
			}
		}

		rc1 := regexp.MustCompile(`^rc[0-9]*$`)
		if rc1 == nil {
			return errors.New("Error on create regex for -rc[0-9]")
		}
		if rc1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "rc", ""))
			if err == nil {
				buildPrefix = "rc"
				goto end
			}
		}

		// Check if version build contains a dot
		dotIdx := strings.LastIndex(gp.VersionBuild, ".")
		if dotIdx > 0 {
			buildPrefix = gp.VersionBuild[0 : dotIdx+1]
			bVersion := gp.VersionBuild[dotIdx+1:]
			buildId, err = strconv.Atoi(bVersion)
			if err == nil {
				goto end
			}
		}

		buildPrefix = gp.VersionBuild + "."
		buildId = 0
	}

end:

	buildId++
	p.Version = fmt.Sprintf("%s%s+%s%d",
		gp.Version, gp.VersionSuffix, buildPrefix, buildId)

	return nil
}
