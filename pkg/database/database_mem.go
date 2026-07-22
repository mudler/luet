// Copyright © 2019-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package database

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/mudler/luet/pkg/api/core/types"
	version "github.com/mudler/luet/pkg/versioner"
	"github.com/pkg/errors"
)

var DBInMemoryInstance = &InMemoryDatabase{
	Mutex:            &sync.Mutex{},
	FileDatabase:     map[string][]string{},
	Database:         map[string]string{},
	CacheNoVersion:   map[string]map[string]interface{}{},
	ProvidesDatabase: map[string]map[string]*types.Package{},
	RevDepsDatabase:  map[string]map[string]*types.Package{},
	cached:           map[string]interface{}{},
}

type InMemoryDatabase struct {
	*sync.Mutex
	Database         map[string]string
	FileDatabase     map[string][]string
	CacheNoVersion   map[string]map[string]interface{}
	ProvidesDatabase map[string]map[string]*types.Package
	RevDepsDatabase  map[string]map[string]*types.Package
	cached           map[string]interface{}

	// noIndex skips building the reverse-dependency, provides and version
	// indexes on insert. See NewInMemoryDatabaseNoIndex.
	noIndex bool
}

func NewInMemoryDatabase(singleton bool) types.PackageDatabase {
	// In memoryDB is a singleton
	if !singleton {
		return &InMemoryDatabase{
			Mutex:            &sync.Mutex{},
			FileDatabase:     map[string][]string{},
			Database:         map[string]string{},
			CacheNoVersion:   map[string]map[string]interface{}{},
			ProvidesDatabase: map[string]map[string]*types.Package{},
			RevDepsDatabase:  map[string]map[string]*types.Package{},
			cached:           map[string]interface{}{},
		}
	}
	return DBInMemoryInstance
}

// NewInMemoryDatabaseNoIndex returns a database that stores packages but builds
// none of the lookup indexes.
//
// Populating those indexes is not free: for each requirement of each inserted
// package it expands a selector across every matching version and deep-copies
// the result, so inserting into a world with long release histories costs far
// more than the insert itself. Profiling an upgrade showed it as the largest
// identifiable cost after garbage collection.
//
// A solver database does not need any of it. It exists so Package.Encode can
// mint a stable SAT variable name and DecodeModel can map a model back to
// packages - a key/value store. Reverse dependencies and version lookups are
// asked of the definition and installed databases, never of this one.
//
// Do NOT use it where FindPackages, FindPackageVersions, GetRevdeps or provides
// resolution are needed: those return nothing here.
func NewInMemoryDatabaseNoIndex() types.PackageDatabase {
	db := NewInMemoryDatabase(false).(*InMemoryDatabase)
	db.noIndex = true
	return db
}

func (db *InMemoryDatabase) Get(s string) (string, error) {
	db.Lock()
	defer db.Unlock()
	pa, ok := db.Database[s]
	if !ok {
		return "", ErrKeyNotFound
	}
	return pa, nil
}

func (db *InMemoryDatabase) Set(k, v string) error {
	db.Lock()
	defer db.Unlock()
	db.Database[k] = v

	return nil
}

func (db *InMemoryDatabase) Create(id string, v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)

	return id, db.Set(id, enc)
}

func (db *InMemoryDatabase) Retrieve(ID string) ([]byte, error) {
	pa, err := db.Get(ID)
	if err != nil {
		return nil, err
	}

	enc, err := base64.StdEncoding.DecodeString(pa)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

func (db *InMemoryDatabase) GetPackage(ID string) (*types.Package, error) {

	enc, err := db.Retrieve(ID)
	if err != nil {
		return nil, err
	}

	p := &types.Package{}

	rawIn := json.RawMessage(enc)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return p, err
	}

	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func (db *InMemoryDatabase) GetAllPackages(packages chan *types.Package) error {
	packs := db.GetPackages()
	for _, p := range packs {
		pack, err := db.GetPackage(p)
		if err != nil {
			return err
		}
		packages <- pack
	}
	return nil
}

func (db *InMemoryDatabase) getRevdeps(p *types.Package, visited map[string]interface{}) (types.Packages, error) {
	var versionsInWorld types.Packages
	if _, ok := visited[p.FullString()]; ok {
		return versionsInWorld, nil
	}
	visited[p.FullString()] = true

	var res types.Packages
	packs, err := db.FindPackages(p)
	if err != nil {
		return res, err
	}

	for _, pp := range packs {
		//	db.Lock()
		list := db.RevDepsDatabase[pp.GetFingerPrint()]
		//	db.Unlock()
		for _, revdep := range list {
			dep, err := db.FindPackage(revdep)
			if err != nil {
				return res, err
			}
			res = append(res, dep)

			packs, err := db.getRevdeps(dep, visited)
			if err != nil {
				return res, err
			}
			res = append(res, packs...)

		}
	}
	return res.Unique(), nil
}

// GetRevdeps returns the package reverse dependencies,
// matching also selectors in versions (>, <, >=, <=)
// TODO: Code should use db explictly
func (db *InMemoryDatabase) GetRevdeps(p *types.Package) (types.Packages, error) {
	return db.getRevdeps(p, make(map[string]interface{}))
}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (db *InMemoryDatabase) CreatePackage(p *types.Package) (string, error) {
	fingerprint := p.GetFingerPrint()

	// Return early if this package is already stored.
	//
	// Package.Encode routes here to obtain a SAT variable name, and formula
	// construction calls it for every literal - in loops quadratic in the number
	// of versions of a package. Marshalling before checking meant the JSON and
	// base64 work was paid per reference rather than once per package, which
	// profiling showed as the dominant cost of resolving a world with long
	// release histories: garbage collection accounted for most of the run.
	//
	// populateCaches is skipped too, consistent with its own guard, which
	// already returns immediately for a fingerprint it has seen.
	db.Lock()
	_, exists := db.Database[fingerprint]
	db.Unlock()
	if exists {
		return fingerprint, nil
	}

	res, err := p.JSON()
	if err != nil {
		return "", err
	}

	ID, err := db.Create(fingerprint, res)
	if err != nil {
		return "", err
	}

	db.populateCaches(p)

	return ID, nil
}

func (db *InMemoryDatabase) updateRevDep(k, v string, b *types.Package) {
	_, ok := db.RevDepsDatabase[k]
	if !ok {
		db.RevDepsDatabase[k] = make(map[string]*types.Package)
	}
	db.RevDepsDatabase[k][v] = b.Clone()
}

func (db *InMemoryDatabase) populateCaches(pd *types.Package) {
	if db.noIndex {
		return
	}

	// Create extra cache between package -> []versions
	db.Lock()
	if db.cached == nil {
		db.cached = map[string]interface{}{}
	}

	if _, ok := db.cached[pd.GetFingerPrint()]; ok {
		db.Unlock()
		return
	}
	db.cached[pd.GetFingerPrint()] = nil

	// Provides: Store package provides, we will reuse this when walking deps
	for _, provide := range pd.Provides {
		if _, ok := db.ProvidesDatabase[provide.GetPackageName()]; !ok {
			db.ProvidesDatabase[provide.GetPackageName()] = make(map[string]*types.Package)

		}

		db.ProvidesDatabase[provide.GetPackageName()][provide.GetVersion()] = pd
	}

	_, ok := db.CacheNoVersion[pd.GetPackageName()]
	if !ok {
		db.CacheNoVersion[pd.GetPackageName()] = make(map[string]interface{})
	}
	db.CacheNoVersion[pd.GetPackageName()][pd.GetVersion()] = nil

	db.Unlock()

	// Updating Revdeps
	// Given that when we populate the cache we don't have the full db at hand
	// We cycle over reverse dependency of a package to update their entry if they are matching
	// the version selector
	db.Lock()
	toUpdate, ok := db.RevDepsDatabase[pd.GetPackageName()]
	if ok {
		for _, pp := range toUpdate {
			for _, re := range pp.GetRequires() {
				if match, _ := pd.VersionMatchSelector(re.GetVersion(), nil); match {
					db.updateRevDep(pd.GetFingerPrint(), pp.GetFingerPrint(), pp)
				}
			}
		}
	}
	db.Unlock()

	for _, re := range pd.GetRequires() {
		packages, _ := db.FindPackages(re)
		db.Lock()
		for _, pa := range packages {
			db.updateRevDep(pa.GetFingerPrint(), pd.GetFingerPrint(), pd)
			db.updateRevDep(pa.GetPackageName(), pd.GetPackageName(), pd)
		}
		db.updateRevDep(re.GetFingerPrint(), pd.GetFingerPrint(), pd)
		db.updateRevDep(re.GetPackageName(), pd.GetPackageName(), pd)
		db.Unlock()
	}
}

func (db *InMemoryDatabase) getProvide(p *types.Package) (*types.Package, error) {

	db.Lock()

	pa, ok := db.ProvidesDatabase[p.GetPackageName()][p.GetVersion()]
	if !ok {
		versions, ok := db.ProvidesDatabase[p.GetPackageName()]
		defer db.Unlock()

		if !ok {
			return nil, ErrNoVersionsFound
		}

		for ve, _ := range versions {

			match, err := p.VersionMatchSelector(ve, nil)
			if err != nil {
				return nil, errors.Wrap(err, "Error on match version")
			}
			if match {
				pa, ok := db.ProvidesDatabase[p.GetPackageName()][ve]
				if !ok {
					return nil, ErrNoVersionsFound
				}
				return pa, nil
			}
		}

		return nil, ErrNoProvider
	}
	db.Unlock()

	return db.FindPackage(pa)
}

func (db *InMemoryDatabase) Clone(to types.PackageDatabase) error {
	return clone(db, to)
}

func (db *InMemoryDatabase) Copy() (types.PackageDatabase, error) {
	return copy(db)
}

func (db *InMemoryDatabase) encodePackage(pd *types.Package) (string, string, error) {
	res, err := pd.JSON()
	if err != nil {
		return "", "", err
	}
	enc := base64.StdEncoding.EncodeToString(res)

	return pd.GetFingerPrint(), enc, nil
}

func (db *InMemoryDatabase) FindPackage(p *types.Package) (*types.Package, error) {

	// Provides: Return the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		return provided, nil
	}

	return db.GetPackage(p.GetFingerPrint())
}

// FindPackages return the list of the packages beloging to cat/name
func (db *InMemoryDatabase) FindPackageVersions(p *types.Package) (types.Packages, error) {
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
	}
	db.Lock()
	versions, ok := db.CacheNoVersion[p.GetPackageName()]
	db.Unlock()
	if !ok {
		return nil, ErrNoVersionsFound
	}
	var versionsInWorld []*types.Package
	for ve, _ := range versions {
		w, err := db.FindPackage(&types.Package{Name: p.GetName(), Category: p.GetCategory(), Version: ve})
		if err != nil {
			return nil, errors.Wrap(err, "Cache mismatch - this shouldn't happen")
		}
		versionsInWorld = append(versionsInWorld, w)
	}

	// CacheNoVersion is a map, so this list was in random order. It feeds the
	// at-most-one clauses that make versions of a package mutually exclusive,
	// and their emission order affects CNF variable numbering.
	SortPackages(versionsInWorld)
	return types.Packages(versionsInWorld), nil
}

// FindPackages return the list of the packages beloging to cat/name (any versions in requested range)
func (db *InMemoryDatabase) FindPackages(p *types.Package) (types.Packages, error) {
	if !p.IsSelector() {
		pack, err := db.FindPackage(p)
		if err != nil {
			return []*types.Package{}, err
		}
		return []*types.Package{pack}, nil
	}
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
		if !provided.IsSelector() {
			return types.Packages{provided}, nil
		}
	}

	db.Lock()
	var matches []*types.Package
	versions, ok := db.CacheNoVersion[p.GetPackageName()]
	for ve := range versions {
		match, _ := p.SelectorMatchVersion(ve, nil)
		if match {
			matches = append(matches, &types.Package{Name: p.GetName(), Category: p.GetCategory(), Version: ve})
		}
	}
	db.Unlock()

	// CacheNoVersion is a map, so the candidate order was random per run. These
	// become the literals of the at-least-one clause for a selector dependency,
	// and their order decides which version the solver reaches first.
	SortPackages(matches)
	if !ok {
		return nil, fmt.Errorf("No versions found for: %s", p.HumanReadableString())
	}
	var versionsInWorld []*types.Package
	for _, p := range matches {
		w, err := db.FindPackage(p)
		if err != nil {
			return nil, errors.Wrap(err, "Cache mismatch - this shouldn't happen")
		}
		versionsInWorld = append(versionsInWorld, w)
	}
	return types.Packages(versionsInWorld), nil
}

func (db *InMemoryDatabase) UpdatePackage(p *types.Package) error {

	_, enc, err := db.encodePackage(p)
	if err != nil {
		return err
	}

	return db.Set(p.GetFingerPrint(), enc)
}

func (db *InMemoryDatabase) GetPackages() []string {
	keys := []string{}
	db.Lock()
	defer db.Unlock()
	for k := range db.Database {
		keys = append(keys, k)
	}
	return keys
}

func (db *InMemoryDatabase) Clean() error {
	db.Database = map[string]string{}
	return nil
}

func (db *InMemoryDatabase) GetPackageFiles(p *types.Package) ([]string, error) {

	db.Lock()
	defer db.Unlock()

	pa, ok := db.FileDatabase[p.GetFingerPrint()]
	if !ok {
		return pa, fmt.Errorf("No key found for: %s", p.HumanReadableString())
	}

	return pa, nil
}
func (db *InMemoryDatabase) SetPackageFiles(p *types.PackageFile) error {
	db.Lock()
	defer db.Unlock()
	db.FileDatabase[p.PackageFingerprint] = p.Files
	return nil
}
func (db *InMemoryDatabase) RemovePackageFiles(p *types.Package) error {
	db.Lock()
	defer db.Unlock()
	delete(db.FileDatabase, p.GetFingerPrint())
	return nil
}

func (db *InMemoryDatabase) RemovePackage(p *types.Package) error {
	db.Lock()
	defer db.Unlock()
	if _, exists := db.CacheNoVersion[p.GetPackageName()]; exists {
		delete(db.CacheNoVersion[p.GetPackageName()], p.GetVersion())
	}
	delete(db.Database, p.GetFingerPrint())
	return nil
}
func (db *InMemoryDatabase) World() types.Packages {
	keys := db.GetPackages()
	all := make([]*types.Package, 0, len(keys))
	// FIXME: This should all be locked in the db - for now forbid the solver to be run in threads.
	for _, k := range keys {
		pack, err := db.GetPackage(k)
		if err == nil {
			all = append(all, pack)
		}
	}

	SortPackages(all)
	return types.Packages(all)
}

// SortPackages orders packages by name and category ascending, then by version
// DESCENDING, and is the ordering the solver sees.
//
// This is load-bearing, not cosmetic. The SAT encoding is version-blind: a
// package becomes an opaque bf.Var("name-category-version") and the at-most-one
// clauses tying versions together are symmetric, so nothing in the formula says
// one version is newer. Which version comes back is decided by gophersat's
// branching, which breaks ties on variable index, which bf assigns in order of
// first appearance during CNF conversion - i.e. the order the world was walked.
// Walking a Go map made that order random per run, so the same input could
// resolve differently, and the solver settled on a needlessly old version most
// of the time.
//
// The DIRECTION matters as much as the determinism. gophersat's default phase
// is false-first, so packages are only installed when propagation forces it,
// and the candidate reached first tends to win. Presenting versions newest-first
// therefore steers it to the newest satisfiable one. Sorting the composite
// fingerprint string in reverse would ALSO be deterministic while reversing name
// order too, which perturbs the CNF onto a different - equally stable - wrong
// answer. Name ascending, version descending.
//
// This is a heuristic, not a guarantee: there is still no optimisation
// objective in the encoding. It is covered by the determinism tests in
// pkg/solver rather than assumed.
func SortPackages(packages []*types.Package) {
	versioner := version.DefaultVersioner()

	sort.SliceStable(packages, func(i, j int) bool {
		a, b := packages[i], packages[j]

		if an, bn := a.GetPackageName(), b.GetPackageName(); an != bn {
			return an < bn
		}

		av, bv := a.GetVersion(), b.GetVersion()
		if av == bv {
			return false
		}

		// Newest first within a package. ValidateSelector is the single
		// version comparator; asking ">b" of a keeps this consistent with
		// Sort and Best rather than introducing a third opinion.
		return versioner.ValidateSelector(av, ">"+bv)
	})
}

func (db *InMemoryDatabase) FindPackageCandidate(p *types.Package) (*types.Package, error) {

	required, err := db.FindPackage(p)
	if err != nil {
		err = nil
		//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
		packages, err := p.Expand(db)
		//	Info("Expanded", packages, err)
		if err != nil || len(packages) == 0 {
			required = p
			err = errors.Wrap(err, "Candidate not found")
		} else {
			required = packages.Best(nil)
		}
		return required, err
		//required = &types.Package{Name: "test"}
	}

	return required, err

}

func (db *InMemoryDatabase) FindPackageLabel(labelKey string) (types.Packages, error) {
	var ans []*types.Package

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}
		if pack.HasLabel(labelKey) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageLabelMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re := regexp.MustCompile(pattern)
	if re == nil {
		return nil, errors.New("Invalid regex " + pattern + "!")
	}

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}
		if pack.MatchLabel(re) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re := regexp.MustCompile(pattern)
	if re == nil {
		return nil, errors.New("Invalid regex " + pattern + "!")
	}

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}

		if re.MatchString(pack.HumanReadableString()) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageByFile(pattern string) (types.Packages, error) {
	return findPackageByFile(db, pattern)
}
