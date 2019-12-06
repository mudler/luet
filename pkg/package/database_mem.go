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
	"sync"
)

var DBInMemoryInstance = &InMemoryDatabase{
	Mutex:        &sync.Mutex{},
	FileDatabase: map[string][]string{},
	Database:     map[string]string{}}

type InMemoryDatabase struct {
	*sync.Mutex
	Database     map[string]string
	FileDatabase map[string][]string
}

func NewInMemoryDatabase(singleton bool) PackageDatabase {
	// In memoryDB is a singleton
	if !singleton {
		return &InMemoryDatabase{
			Mutex:        &sync.Mutex{},
			FileDatabase: map[string][]string{},
			Database:     map[string]string{}}
	}
	return DBInMemoryInstance
}

func (db *InMemoryDatabase) Get(s string) (string, error) {
	db.Lock()
	defer db.Unlock()
	pa, ok := db.Database[s]
	if !ok {
		return "", errors.New("No key found with that id")
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

func (db *InMemoryDatabase) GetPackage(ID string) (Package, error) {

	enc, err := db.Retrieve(ID)
	if err != nil {
		return nil, err
	}

	p := &DefaultPackage{}

	rawIn := json.RawMessage(enc)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return &DefaultPackage{}, err
	}

	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func (db *InMemoryDatabase) GetAllPackages(packages chan Package) error {
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

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (db *InMemoryDatabase) CreatePackage(p Package) (string, error) {
	pd, ok := p.(*DefaultPackage)
	if !ok {
		return "", errors.New("InMemoryDatabase suports only DefaultPackage")
	}

	res, err := pd.JSON()
	if err != nil {
		return "", err
	}

	ID, err := db.Create(pd.GetFingerPrint(), res)
	if err != nil {
		return "", err
	}
	return ID, nil
}

func (db *InMemoryDatabase) encodePackage(p Package) (string, string, error) {
	pd, ok := p.(*DefaultPackage)
	if !ok {
		return "", "", errors.New("InMemoryDatabase suports only DefaultPackage")
	}

	res, err := pd.JSON()
	if err != nil {
		return "", "", err
	}
	enc := base64.StdEncoding.EncodeToString(res)

	return p.GetFingerPrint(), enc, nil
}

func (db *InMemoryDatabase) FindPackage(p Package) (Package, error) {
	return db.GetPackage(p.GetFingerPrint())
}

func (db *InMemoryDatabase) UpdatePackage(p Package) error {

	_, enc, err := db.encodePackage(p)
	if err != nil {
		return err
	}

	return db.Set(p.GetFingerPrint(), enc)

	return errors.New("Package not found")
}

func (db *InMemoryDatabase) GetPackages() []string {
	keys := []string{}
	db.Lock()
	defer db.Unlock()
	for k, _ := range db.Database {
		keys = append(keys, k)
	}
	return keys
}

func (db *InMemoryDatabase) Clean() error {
	db.Database = map[string]string{}
	return nil
}

func (db *InMemoryDatabase) GetPackageFiles(p Package) ([]string, error) {

	db.Lock()
	defer db.Unlock()

	pa, ok := db.FileDatabase[p.GetFingerPrint()]
	if !ok {
		return pa, errors.New("No key found with that id")
	}

	return pa, nil
}
func (db *InMemoryDatabase) SetPackageFiles(p *PackageFile) error {
	db.Lock()
	defer db.Unlock()
	db.FileDatabase[p.PackageFingerprint] = p.Files
	return nil
}
func (db *InMemoryDatabase) RemovePackageFiles(p Package) error {
	db.Lock()
	defer db.Unlock()
	delete(db.FileDatabase, p.GetFingerPrint())
	return nil
}

func (db *InMemoryDatabase) RemovePackage(p Package) error {
	db.Lock()
	defer db.Unlock()

	delete(db.Database, p.GetFingerPrint())
	return nil
}
func (db *InMemoryDatabase) World() []Package {
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

func (db *InMemoryDatabase) FindPackageCandidate(p Package) (Package, error) {

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
