// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

import "go.etcd.io/bbolt"

type schemaMigration func(tx *bbolt.Tx) error

var migrations = []schemaMigration{migrateDefaultPackage}

var migrateDefaultPackage schemaMigration = func(tx *bbolt.Tx) error {
	// previously we had pkg.DefaultPackage
	// IF it's there, rename it to the proper bucket
	b := tx.Bucket([]byte("DefaultPackage"))
	if b != nil {
		newB, err := tx.CreateBucket([]byte("Package"))
		if err != nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			return newB.Put(k, v)
		})

		tx.DeleteBucket([]byte("DefaultPackage"))
	}
	return nil
}
