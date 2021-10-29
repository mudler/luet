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

package artifact

import (

	//"strconv"

	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"sort"

	mtree "github.com/vbatts/go-mtree"

	//	. "github.com/mudler/luet/pkg/logger"
	containerdCompression "github.com/containerd/containerd/archive/compression"
	"github.com/pkg/errors"
)

type HashImplementation string

const (
	// SHA256 Implementation
	SHA256 HashImplementation = "sha256"
	// MTREE Implementation
	MTREE HashImplementation = "mtree"
)

// FileHashing is the hashing set reserved to files
var FileHashing = []HashImplementation{SHA256}

// TarHashing is the hashing set reserved to archives
var TarHashing = []HashImplementation{SHA256, MTREE}

// default set
var defaultHashing = []HashImplementation{SHA256, MTREE}

var mtreeKeywords []mtree.Keyword = []mtree.Keyword{
	"type",
	"sha512digest",
}

type Checksums map[string]string

type HashOptions struct {
	Hasher hash.Hash
	Type   HashImplementation
}

func (c Checksums) List() (res [][]string) {
	keys := make([]string, 0)
	for k, _ := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		res = append(res, []string{k, c[k]})
	}
	return
}

func (c Checksums) Only(t ...HashImplementation) Checksums {
	newc := Checksums{}

	for k, v := range c {
		if Hashes(t).Exist(HashImplementation(k)) {
			newc[k] = v
		}
	}
	return newc
}

type Hashes []HashImplementation

func (h Hashes) Exist(t HashImplementation) bool {
	for _, tt := range h {
		if tt == t {
			return true
		}
	}
	return false
}

// Generate generates all Checksums supported for the artifact
func (c *Checksums) Generate(a *PackageArtifact, t ...HashImplementation) (err error) {
	f, err := os.Open(a.Path)
	if err != nil {
		return err
	}

	if len(t) == 0 {
		t = defaultHashing
	}

	for _, h := range t {
		sum, err := h.Sum(f)
		if err != nil {
			return err
		}
		(*c)[string(h)] = sum
	}
	return
}

func (c Checksums) Compare(d Checksums) error {
	for t, sum := range d {
		if t == string(MTREE) {
			sum2, exists := c[t]
			if !exists {
				continue
			}

			b1, err := base64.RawStdEncoding.DecodeString(sum)
			if err != nil {
				return err
			}

			b2, err := base64.RawStdEncoding.DecodeString(sum2)
			if err != nil {
				return err
			}
			spec, err := mtree.ParseSpec(bytes.NewReader(b1))
			if err != nil {
				return err
			}
			spec2, err := mtree.ParseSpec(bytes.NewReader(b2))
			if err != nil {
				return err
			}

			res, err := mtree.Compare(spec, spec2, mtreeKeywords)
			if err != nil {
				return err
			}

			if len(res) != 0 {
				return errors.New("MTREE mismatch")
			}

		} else {
			if v, ok := c[t]; ok && v != sum {
				return errors.New("Checksum mismsatch")
			}
		}
	}
	return nil
}

func (t HashImplementation) Sum(r io.ReadCloser) (sum string, err error) {
	//	defer r.Close()
	switch t {
	case SHA256:
		hasher := sha256.New()
		_, err = io.Copy(hasher, r)
		if err != nil {
			return
		}

		sum = fmt.Sprintf("%x", hasher.Sum(nil))

	case MTREE:
		sum, err = mtreeSum(r)
		sum = base64.RawStdEncoding.EncodeToString([]byte(sum))
		return
	}

	return
}

func mtreeSum(r io.ReadCloser) (string, error) {
	decompressed, err := containerdCompression.DecompressStream(r)
	if err != nil {
		return "", errors.Wrap(err, "Cannot open stream")
	}

	ts := mtree.NewTarStreamer(decompressed, []mtree.ExcludeFunc{}, mtreeKeywords)
	if _, err := io.Copy(ioutil.Discard, ts); err != nil && err != io.EOF {
		return "", err
	}
	if err := ts.Close(); err != nil {
		return "", err
	}

	stateDh, err := ts.Hierarchy()
	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString("")
	_, err = stateDh.WriteTo(buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
