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

package compiler

import (

	//"strconv"

	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"

	//	. "github.com/mudler/luet/pkg/logger"
	"github.com/pkg/errors"
)

type HashImplementation string

const (
	SHA256 HashImplementation = "sha256"
)

type Checksums map[string]string

type HashOptions struct {
	Hasher hash.Hash
	Type   HashImplementation
}

// Generate generates all Checksums supported for the artifact
func (c *Checksums) Generate(a Artifact) error {
	return c.generateSHA256(a)
}

func (c Checksums) Compare(d Checksums) error {
	for t, sum := range d {
		if v, ok := c[t]; ok && v != sum {
			return errors.New("Checksum mismsatch")
		}
	}
	return nil
}

func (c *Checksums) generateSHA256(a Artifact) error {
	return c.generateSum(a, HashOptions{Hasher: sha256.New(), Type: SHA256})
}

func (c *Checksums) generateSum(a Artifact, opts HashOptions) error {

	f, err := os.Open(a.GetPath())
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(opts.Hasher, f); err != nil {
		return err
	}

	sum := fmt.Sprintf("%x", opts.Hasher.Sum(nil))

	(*c)[string(opts.Type)] = sum
	return nil
}
