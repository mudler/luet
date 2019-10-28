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
)

var DBInMemoryInstance PackageDatabase

type InMemoryDatabase struct {
	Database map[string]string
}

func NewInMemoryDatabase() PackageDatabase {
	// In memoryDB is a singleton
	if DBInMemoryInstance == nil {
		DBInMemoryInstance = &InMemoryDatabase{Database: map[string]string{}}
	}
	return DBInMemoryInstance
}

func (db *InMemoryDatabase) Get(s string) (string, error) {

	pa, ok := db.Database[s]
	if !ok {
		return "", errors.New("No key found with that id")
	}
	return pa, nil
}

func (db *InMemoryDatabase) Set(k, v string) error {
	db.Database[k] = v

	return nil
}

func (db *InMemoryDatabase) Create(v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)
	crc32q := crc32.MakeTable(0xD5828281)
	ID := fmt.Sprintf("%08x", crc32.Checksum([]byte(enc), crc32q))

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

func (db *InMemoryDatabase) FindPackage(name, version string) (Package, error) {
	return nil, errors.New("Not implemented")
}

func (db *InMemoryDatabase) UpdatePackage(p Package) error {
	return errors.New("Not implemented")
}

func (db *InMemoryDatabase) GetPackages() []string {
	return []string{}
}
