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
	"os"
	"strconv"
	"sync"
	"time"

	version "github.com/hashicorp/go-version"
	"github.com/pkg/errors"

	storm "github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"go.etcd.io/bbolt"
)

//var BoltInstance PackageDatabase

type BoltDatabase struct {
	sync.Mutex
	Path             string
	ProvidesDatabase map[string]map[string]Package
}

func NewBoltDatabase(path string) PackageDatabase {
	// if BoltInstance == nil {
	// 	BoltInstance = &BoltDatabase{Path: path}
	// }
	//return BoltInstance, nil
	return &BoltDatabase{Path: path, ProvidesDatabase: map[string]map[string]Package{}}
}

func (db *BoltDatabase) Get(s string) (string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return "", err
	}
	defer bolt.Close()
	var str string
	bolt.Get("solver", s, &str)

	return str, errors.New("Not implemented")
}

func (db *BoltDatabase) Set(k, v string) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return err
	}
	defer bolt.Close()
	return bolt.Set("solver", k, v)
}
func (db *BoltDatabase) Create(id string, v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)

	return id, db.Set(id, enc)
}

func (db *BoltDatabase) Retrieve(ID string) ([]byte, error) {
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

func (db *BoltDatabase) FindPackage(tofind Package) (Package, error) {
	// Provides: Return the replaced package here
	if provided, err := db.getProvide(tofind); err == nil {
		return provided, nil
	}

	p := &DefaultPackage{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return nil, err
	}
	defer bolt.Close()

	err = bolt.Select(q.Eq("Name", tofind.GetName()), q.Eq("Category", tofind.GetCategory()), q.Eq("Version", tofind.GetVersion())).Limit(1).First(p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *BoltDatabase) UpdatePackage(p Package) error {
	// TODO: Change, but by query we cannot update by ID
	err := db.RemovePackage(p)
	if err != nil {
		return err
	}
	_, err = db.CreatePackage(p)

	if err != nil {
		return err
	}

	return nil
}

func (db *BoltDatabase) GetPackage(ID string) (Package, error) {
	p := &DefaultPackage{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return nil, err
	}
	defer bolt.Close()
	iid, err := strconv.Atoi(ID)
	if err != nil {
		return nil, err
	}
	err = bolt.Select(q.Eq("ID", iid)).Limit(1).First(p)

	//err = bolt.One("id", iid, p)
	return p, err
}

func (db *BoltDatabase) GetPackages() []string {
	ids := []string{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return []string{}
	}
	defer bolt.Close()
	// Fetching records one by one (useful when the bucket contains a lot of records)
	query := bolt.Select()

	query.Each(new(DefaultPackage), func(record interface{}) error {
		u := record.(*DefaultPackage)
		ids = append(ids, strconv.Itoa(u.ID))
		return nil
	})
	return ids
}

func (db *BoltDatabase) GetAllPackages(packages chan Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return err
	}
	defer bolt.Close()
	// Fetching records one by one (useful when the bucket contains a lot of records)
	//query := bolt.Select()

	var packs []Package
	err = bolt.All(&packs)
	if err != nil {
		return err
	}

	for _, r := range packs {
		packages <- r
	}

	return nil

	// return query.Each(new(DefaultPackage), func(record interface{}) error {
	// 	u := record.(*DefaultPackage)
	// 	packages <- u
	// 	return err
	// })
}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (db *BoltDatabase) CreatePackage(p Package) (string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return "", errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	dp, ok := p.(*DefaultPackage)
	if !ok {
		return "", errors.New("Bolt DB support only DefaultPackage type for now")
	}

	err = bolt.Save(dp)
	if err != nil {
		return "", errors.Wrap(err, "Error saving package to "+db.Path)
	}

	// Create extra cache between package -> []versions
	db.Lock()
	defer db.Unlock()
	// TODO: Replace with a bolt implementation (and not in memory)
	// Provides: Store package provides, we will reuse this when walking deps
	for _, provide := range dp.Provides {
		if _, ok := db.ProvidesDatabase[provide.GetPackageName()]; !ok {
			db.ProvidesDatabase[provide.GetPackageName()] = make(map[string]Package)

		}

		db.ProvidesDatabase[provide.GetPackageName()][provide.GetVersion()] = p
	}

	return strconv.Itoa(dp.ID), err
}

// Dup from memory implementation
func (db *BoltDatabase) getProvide(p Package) (Package, error) {
	db.Lock()
	pa, ok := db.ProvidesDatabase[p.GetPackageName()][p.GetVersion()]
	if !ok {
		versions, ok := db.ProvidesDatabase[p.GetPackageName()]
		db.Unlock()

		if !ok {
			return nil, errors.New("No versions found for package")
		}

		for ve, _ := range versions {

			v, err := version.NewVersion(p.GetVersion())
			if err != nil {
				return nil, err
			}
			constraints, err := version.NewConstraint(ve)
			if err != nil {
				return nil, err
			}

			if constraints.Check(v) {
				pa, ok := db.ProvidesDatabase[p.GetPackageName()][ve]
				if !ok {
					return nil, errors.New("No versions found for package")
				}
				return pa, nil //pick the first (we shouldn't have providers that are conflicting)
				// TODO: A find dbcall here would recurse, but would give chance to have providers of providers
			}
		}

		return nil, errors.New("No package provides this")
	}
	db.Unlock()
	return db.FindPackage(pa)
}

func (db *BoltDatabase) Clean() error {
	db.Lock()
	defer db.Unlock()
	return os.RemoveAll(db.Path)
}

func (db *BoltDatabase) GetPackageFiles(p Package) ([]string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return []string{}, errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	var pf PackageFile
	err = files.One("PackageFingerprint", p.GetFingerPrint(), &pf)
	if err != nil {
		return []string{}, errors.Wrap(err, "While finding files")
	}
	return pf.Files, nil
}
func (db *BoltDatabase) SetPackageFiles(p *PackageFile) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	return files.Save(p)
}
func (db *BoltDatabase) RemovePackageFiles(p Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	var pf PackageFile
	err = files.One("PackageFingerprint", p.GetFingerPrint(), &pf)
	if err != nil {
		return errors.Wrap(err, "While finding files")
	}
	return files.DeleteStruct(&pf)
}

func (db *BoltDatabase) RemovePackage(p Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()
	var found DefaultPackage
	err = bolt.Select(q.Eq("Name", p.GetName()), q.Eq("Category", p.GetCategory()), q.Eq("Version", p.GetVersion())).Limit(1).Delete(&found)
	if err != nil {
		return errors.Wrap(err, "No package found to delete")
	}
	return nil
}

func (db *BoltDatabase) World() []Package {

	var all []Package
	// FIXME: This should all be locked in the db - for now forbid the solver to be run in threads.
	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err == nil {
			all = append(all, pack)
		}
	}
	return all
}

func (db *BoltDatabase) FindPackageCandidate(p Package) (Package, error) {

	required, err := db.FindPackage(p)
	if err != nil {

		//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
		packages, err := p.Expand(db)
		//	Info("Expanded", packages, err)
		if err != nil || len(packages) == 0 {
			required = p
		} else {
			required = Best(packages)

		}
		return required, nil
		//required = &DefaultPackage{Name: "test"}
	}

	return required, err

}

// FindPackages return the list of the packages beloging to cat/name  (any versions in requested range)
// FIXME: Optimize, see inmemorydb
func (db *BoltDatabase) FindPackages(p Package) ([]Package, error) {
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
	}
	var versionsInWorld []Package
	for _, w := range db.World() {
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

// FindPackageVersions return the list of the packages beloging to cat/name
func (db *BoltDatabase) FindPackageVersions(p Package) ([]Package, error) {
	var versionsInWorld []Package
	for _, w := range db.World() {
		if w.GetName() != p.GetName() || w.GetCategory() != p.GetCategory() {
			continue
		}

		versionsInWorld = append(versionsInWorld, w)
	}
	return versionsInWorld, nil
}
