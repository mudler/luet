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
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	storm "github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"go.etcd.io/bbolt"
)

//var BoltInstance PackageDatabase

type BoltDatabase struct {
	sync.Mutex
	Path string
}

func NewBoltDatabase(path string) PackageDatabase {
	// if BoltInstance == nil {
	// 	BoltInstance = &BoltDatabase{Path: path}
	// }
	//return BoltInstance, nil
	return &BoltDatabase{Path: path}
}

func (db *BoltDatabase) Get(s string) (string, error) {
	return "", errors.New("Not implemented")
}

func (db *BoltDatabase) Set(k, v string) error {
	return errors.New("Not implemented")

}

func (db *BoltDatabase) Create(v []byte) (string, error) {
	return "", errors.New("Not implemented")
}

func (db *BoltDatabase) Retrieve(ID string) ([]byte, error) {
	return []byte{}, errors.New("Not implemented")
}

func (db *BoltDatabase) FindPackage(tofind Package) (Package, error) {
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

	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return err
	}
	defer bolt.Close()

	dp, ok := p.(*DefaultPackage)
	if !ok {
		return errors.New("Bolt DB support only DefaultPackage type for now")
	}
	err = bolt.Update(dp)
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
	err = bolt.One("ID", iid, p)
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

	return strconv.Itoa(dp.ID), err
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
func (db *BoltDatabase) SetPackageFiles(p PackageFile) error {
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
	p, err = db.FindPackage(p)
	if err != nil {
		return errors.Wrap(err, "No package found")
	}
	return bolt.DeleteStruct(p)
}
