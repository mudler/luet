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
	"sync"
)

var DBInMemoryInstance PackageDatabase

type InMemoryDatabase struct {
	*sync.Mutex
	Database map[string]string
}

func NewInMemoryDatabase(singleton bool) PackageDatabase {
	// In memoryDB is a singleton
	if singleton && DBInMemoryInstance == nil {
		DBInMemoryInstance = &InMemoryDatabase{
			Mutex: &sync.Mutex{},

			Database: map[string]string{}}
	}
	if !singleton {
		return &InMemoryDatabase{
			Mutex: &sync.Mutex{},

			Database: map[string]string{}}
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

func (db *InMemoryDatabase) Create(v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)
	crc32q := crc32.MakeTable(0xD5828281)
	ID := fmt.Sprintf("%08x", crc32.Checksum([]byte(enc), crc32q)) // TODO: Replace with package fingerprint?

	return ID, db.Set(ID, enc)
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

	if err := json.Unmarshal(enc, &p); err != nil {
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

	res, err := json.Marshal(pd)
	if err != nil {
		return "", err
	}

	ID, err := db.Create(res)
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

	res, err := json.Marshal(pd)
	if err != nil {
		return "", "", err
	}
	enc := base64.StdEncoding.EncodeToString(res)
	crc32q := crc32.MakeTable(0xD5828281)
	ID := fmt.Sprintf("%08x", crc32.Checksum([]byte(enc), crc32q)) // TODO: Replace with package fingerprint?

	return ID, enc, nil
}

func (db *InMemoryDatabase) FindPackage(p Package) (Package, error) {

	// TODO: Replace this piece, when IDs are fingerprint, findpackage becames O(1)

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return nil, err
		}
		if pack.Matches(p) {
			return pack, nil
		}
	}
	return nil, errors.New("Package not found")
}

func (db *InMemoryDatabase) UpdatePackage(p Package) error {
	var id string
	found := false
	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return err
		}
		if pack.Matches(p) {
			id = k
			found = true
			break
		}
	}
	if found {

		_, enc, err := db.encodePackage(p)
		if err != nil {
			return err
		}

		return db.Set(id, enc)
	}
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
