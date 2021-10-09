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
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/mudler/luet/pkg/helpers/docker"
	"github.com/mudler/luet/pkg/helpers/match"
	version "github.com/mudler/luet/pkg/versioner"

	gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	"github.com/crillab/gophersat/bf"
	"github.com/ghodss/yaml"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

// Package is a package interface (TBD)
// FIXME: Currently some of the methods are returning DefaultPackages due to JSON serialization of the package
type Package interface {
	Encode(PackageDatabase) (string, error)
	Related(definitiondb PackageDatabase) Packages

	BuildFormula(PackageDatabase, PackageDatabase) ([]bf.Formula, error)

	GetFingerPrint() string
	GetPackageName() string
	ImageID() string
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
	SetName(string)
	GetCategory() string

	GetVersion() string
	SetVersion(string)
	RequiresContains(PackageDatabase, Package) (bool, error)
	Matches(m Package) bool
	AtomMatches(m Package) bool
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

	AddAnnotation(string, string)
	GetAnnotations() map[string]string
	HasAnnotation(string) bool
	MatchAnnotation(*regexp.Regexp) bool

	IsHidden() bool
	IsSelector() bool
	VersionMatchSelector(string, version.Versioner) (bool, error)
	SelectorMatchVersion(string, version.Versioner) (bool, error)

	String() string
	HumanReadableString() string
	HashFingerprint(string) string

	SetBuildTimestamp(s string)
	GetBuildTimestamp() string

	Clone() Package

	GetMetadataFilePath() string
	SetTreeDir(s string)
	GetTreeDir() string

	Mark() Package

	JSON() ([]byte, error)
}

const (
	PackageMetaSuffix     = "metadata.yaml"
	PackageCollectionFile = "collection.yaml"
	PackageDefinitionFile = "definition.yaml"
)

type Tree interface {
	GetPackageSet() PackageDatabase
	Prelude() string // A tree might have a prelude to be able to consume a tree
	SetPackageSet(s PackageDatabase)
	World() (Packages, error)
	FindPackage(Package) (Package, error)
}

type Packages []Package

type DefaultPackages []*DefaultPackage

type PackageMap map[string]Package

func (pm PackageMap) String() string {
	rr := []string{}
	for _, r := range pm {

		rr = append(rr, r.HumanReadableString())

	}
	return fmt.Sprint(rr)
}

func (d DefaultPackages) Hash(salt string) string {

	overallFp := ""
	for _, c := range d {
		overallFp = overallFp + c.HashFingerprint("join")
	}
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", overallFp, salt))
	return fmt.Sprintf("%x", h.Sum(nil))
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

type rawPackages []map[string]interface{}

func (r rawPackages) Find(name, category, version string) map[string]interface{} {
	for _, v := range r {
		if v["name"] == name && v["category"] == category && v["version"] == version {
			return v
		}
	}
	return map[string]interface{}{}
}

func GetRawPackages(yml []byte) (rawPackages, error) {
	var rawPackages struct {
		Packages []map[string]interface{} `yaml:"packages"`
	}
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []map[string]interface{}{}, err
	}
	err = json.Unmarshal(bytes, &rawPackages)
	if err != nil {
		return []map[string]interface{}{}, err
	}
	return rawPackages.Packages, nil

}

type Collection struct {
	Packages []DefaultPackage `json:"packages"`
}

func DefaultPackagesFromYAML(yml []byte) ([]DefaultPackage, error) {

	var unescaped Collection
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []DefaultPackage{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []DefaultPackage{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return []DefaultPackage{}, err
	}
	return unescaped.Packages, nil
}

// Major and minor gets escaped when marshalling in JSON, making compiler fails recognizing selectors for expansion
func (t *DefaultPackage) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// GetMetadataFilePath returns the canonical name of an artifact metadata file
func (d *DefaultPackage) GetMetadataFilePath() string {
	return fmt.Sprintf("%s.%s", d.GetFingerPrint(), PackageMetaSuffix)
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
	Provides         []*DefaultPackage `json:"provides,omitempty"` // Affects YAML field names too.
	Hidden           bool              `json:"hidden,omitempty"`   // Affects YAML field names too.

	// Annotations are used for core features/options
	Annotations map[string]string `json:"annotations,omitempty"` // Affects YAML field names too

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty"`

	Description    string   `json:"description,omitempty"`
	Uri            []string `json:"uri,omitempty"`
	License        string   `json:"license,omitempty"`
	BuildTimestamp string   `json:"buildtimestamp,omitempty"`

	Labels map[string]string `json:"labels,omitempty"` // Affects YAML field names too.

	TreeDir string `json:"treedir,omitempty"`
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
		Labels:           nil,
	}
}

func (p *DefaultPackage) SetTreeDir(s string) {
	p.TreeDir = s
}
func (p *DefaultPackage) GetTreeDir() string {
	return p.TreeDir
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

func (p *DefaultPackage) HashFingerprint(salt string) string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", p.GetFingerPrint(), salt))
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

func (p *DefaultPackage) ImageID() string {
	return docker.StripInvalidStringsFromImage(p.GetFingerPrint())
}

// GetBuildTimestamp returns the package build timestamp
func (p *DefaultPackage) GetBuildTimestamp() string {
	return p.BuildTimestamp
}

// SetBuildTimestamp sets the package Build timestamp
func (p *DefaultPackage) SetBuildTimestamp(s string) {
	p.BuildTimestamp = s
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

func (p *DefaultPackage) IsHidden() bool {
	return p.Hidden
}

func (p *DefaultPackage) HasLabel(label string) bool {
	return match.MapHasKey(&p.Labels, label)
}

func (p *DefaultPackage) MatchLabel(r *regexp.Regexp) bool {
	return match.MapMatchRegex(&p.Labels, r)
}

func (p DefaultPackage) IsCollection() bool {
	return fileHelper.Exists(filepath.Join(p.Path, PackageCollectionFile))
}

func (p *DefaultPackage) HasAnnotation(label string) bool {
	return match.MapHasKey(&p.Annotations, label)
}

func (p *DefaultPackage) MatchAnnotation(r *regexp.Regexp) bool {
	return match.MapMatchRegex(&p.Annotations, r)
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

func (p *DefaultPackage) GetName() string {
	return p.Name
}

func (p *DefaultPackage) GetVersion() string {
	return p.Version
}
func (p *DefaultPackage) SetVersion(v string) {
	p.Version = v
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

func (p *DefaultPackage) SetName(s string) {
	p.Name = s
}

func (p *DefaultPackage) GetUses() []string {
	return p.UseFlags
}
func (p *DefaultPackage) AddLabel(k, v string) {
	if p.Labels == nil {
		p.Labels = make(map[string]string, 0)
	}
	p.Labels[k] = v
}
func (p *DefaultPackage) AddAnnotation(k, v string) {
	if p.Annotations == nil {
		p.Annotations = make(map[string]string, 0)
	}
	p.Annotations[k] = v
}
func (p *DefaultPackage) GetLabels() map[string]string {
	return p.Labels
}
func (p *DefaultPackage) GetAnnotations() map[string]string {
	return p.Annotations
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

func (p *DefaultPackage) AtomMatches(m Package) bool {
	if p.GetName() == m.GetName() && p.GetCategory() == m.GetCategory() {
		return true
	}
	return false
}

func (p *DefaultPackage) Mark() Package {
	marked := p.Clone()
	marked.SetName("@@" + marked.GetName())
	return marked
}

func (p *DefaultPackage) Expand(definitiondb PackageDatabase) (Packages, error) {
	var versionsInWorld Packages

	all, err := definitiondb.FindPackages(p)
	if err != nil {
		return nil, err
	}
	for _, w := range all {
		match, err := p.SelectorMatchVersion(w.GetVersion(), nil)
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

func walkPackage(p Package, definitiondb PackageDatabase, visited map[string]interface{}) Packages {
	var versionsInWorld Packages
	if _, ok := visited[p.HumanReadableString()]; ok {
		return versionsInWorld
	}
	visited[p.HumanReadableString()] = true

	revdeps, _ := definitiondb.GetRevdeps(p)
	for _, r := range revdeps {
		versionsInWorld = append(versionsInWorld, r)
	}

	if !p.IsSelector() {
		versionsInWorld = append(versionsInWorld, p)
	}

	for _, re := range p.GetRequires() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)
		}

	}
	for _, re := range p.GetConflicts() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)

		}
	}
	return versionsInWorld.Unique()
}

func (p *DefaultPackage) Related(definitiondb PackageDatabase) Packages {
	return walkPackage(p, definitiondb, map[string]interface{}{})
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

func (pack *DefaultPackage) scanRequires(definitiondb PackageDatabase, s Package, visited map[string]interface{}) (bool, error) {
	if _, ok := visited[pack.HumanReadableString()]; ok {
		return false, nil
	}
	visited[pack.HumanReadableString()] = true
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
		if contains, err := re.scanRequires(definitiondb, s, visited); err == nil && contains {
			return true, nil
		}
	}

	return false, nil
}

// RequiresContains recursively scans into the database packages dependencies to find a match with the given package
// It is used by the solver during uninstall.
func (pack *DefaultPackage) RequiresContains(definitiondb PackageDatabase, s Package) (bool, error) {
	return pack.scanRequires(definitiondb, s, make(map[string]interface{}))
}

// Best returns the best version of the package (the most bigger) from a list
// Accepts a versioner interface to change the ordering policy. If null is supplied
// It defaults to version.WrappedVersioner which supports both semver and debian versioning
func (set Packages) Best(v version.Versioner) Package {
	if v == nil {
		v = &version.WrappedVersioner{}
	}
	var versionsMap map[string]Package = make(map[string]Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		versionsRaw = append(versionsRaw, p.GetVersion())
		versionsMap[p.GetVersion()] = p
	}
	sorted := v.Sort(versionsRaw)

	return versionsMap[sorted[len(sorted)-1]]
}

func (set Packages) Find(packageName string) (Package, error) {
	for _, p := range set {
		if p.GetPackageName() == packageName {
			return p, nil
		}
	}

	return &DefaultPackage{}, errors.New("package not found")
}

func (set Packages) Unique() Packages {
	var result Packages
	uniq := make(map[string]Package)
	for _, p := range set {
		uniq[p.GetFingerPrint()] = p
	}
	for _, p := range uniq {
		result = append(result, p)
	}
	return result
}

func (p *DefaultPackage) GetRuntimePackage() (*DefaultPackage, error) {
	var r *DefaultPackage
	if p.IsCollection() {
		collectionFile := filepath.Join(p.Path, PackageCollectionFile)
		dat, err := ioutil.ReadFile(collectionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", collectionFile)
		}
		coll, err := DefaultPackagesFromYAML(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", collectionFile)
		}
		for _, c := range coll {
			if c.Matches(p) {
				r = &c
				break
			}
		}
	} else {
		definitionFile := filepath.Join(p.Path, PackageDefinitionFile)
		dat, err := ioutil.ReadFile(definitionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", definitionFile)
		}
		d, err := DefaultPackageFromYaml(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", definitionFile)
		}
		r = &d
	}
	return r, nil
}

func (pack *DefaultPackage) buildFormula(definitiondb PackageDatabase, db PackageDatabase, visited map[string]interface{}) ([]bf.Formula, error) {
	if _, ok := visited[pack.HumanReadableString()]; ok {
		return nil, nil
	}
	visited[pack.HumanReadableString()] = true
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

				var ALO []bf.Formula // , priorityConstraints, priorityALO []bf.Formula

				// Try to prio best match
				// Force the solver to consider first our candidate (if does exists).
				// Then builds ALO and AMO for the requires.
				// c, candidateErr := definitiondb.FindPackageCandidate(requiredDef)
				// var C bf.Formula
				// if candidateErr == nil {
				// 	// We have a desired candidate, try to look a solution with that included first
				// 	for _, o := range packages {
				// 		encodedB, err := o.Encode(db)
				// 		if err != nil {
				// 			return nil, err
				// 		}
				// 		B := bf.Var(encodedB)
				// 		if !o.Matches(c) {
				// 			priorityConstraints = append(priorityConstraints, bf.Not(B))
				// 			priorityALO = append(priorityALO, B)
				// 		}
				// 	}
				// 	encodedC, err := c.Encode(db)
				// 	if err != nil {
				// 		return nil, err
				// 	}
				// 	C = bf.Var(encodedC)
				// 	// Or the Candidate is true, or all the others might be not true
				// 	// This forces the CDCL sat implementation to look first at a solution with C=true
				// 	//formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.And(C, bf.Or(priorityConstraints...)), bf.And(bf.Not(C), bf.Or(priorityALO...)))))
				// 	formulas = append(formulas, bf.Or(C, bf.Or(priorityConstraints...)))
				// }

				// AMO/ALO - At most/least one
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
		r := required.(*DefaultPackage) // We know since the implementation is DefaultPackage, that can be only DefaultPackage
		f, err := r.buildFormula(definitiondb, db, visited)
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
						r := p.(*DefaultPackage) // We know since the implementation is DefaultPackage, that can be only DefaultPackage
						f, err := r.buildFormula(definitiondb, db, visited)
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

		r := required.(*DefaultPackage) // We know since the implementation is DefaultPackage, that can be only DefaultPackage
		f, err := r.buildFormula(definitiondb, db, visited)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)

	}

	return formulas, nil
}

func (pack *DefaultPackage) BuildFormula(definitiondb PackageDatabase, db PackageDatabase) ([]bf.Formula, error) {
	return pack.buildFormula(definitiondb, db, make(map[string]interface{}))
}

func (p *DefaultPackage) Explain() {

	fmt.Println("====================")
	fmt.Println("Name: ", p.GetName())
	fmt.Println("Category: ", p.GetCategory())
	fmt.Println("Version: ", p.GetVersion())

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

func (p *DefaultPackage) SelectorMatchVersion(ver string, v version.Versioner) (bool, error) {
	if !p.IsSelector() {
		return false, errors.New("Package is not a selector")
	}
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(ver, p.GetVersion()), nil
}

func (p *DefaultPackage) VersionMatchSelector(selector string, v version.Versioner) (bool, error) {
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(p.GetVersion(), selector), nil
}
