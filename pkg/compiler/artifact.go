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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	. "github.com/mudler/luet/pkg/logger"
	"github.com/mudler/luet/pkg/solver"

	"github.com/mudler/luet/pkg/helpers"
	"github.com/pkg/errors"
)

type PackageArtifact struct {
	Path         string
	Dependencies []Artifact
	CompileSpec  CompilationSpec

	SourceAssertion solver.PackagesAssertions
}

func NewPackageArtifact(path string) Artifact {
	return &PackageArtifact{Path: path, Dependencies: []Artifact{}}
}

func (a *PackageArtifact) GetSourceAssertion() solver.PackagesAssertions {
	return a.SourceAssertion
}

func (a *PackageArtifact) SetCompileSpec(as CompilationSpec) {
	a.CompileSpec = as
}

func (a *PackageArtifact) GetCompileSpec() CompilationSpec {
	return a.CompileSpec
}

func (a *PackageArtifact) SetSourceAssertion(as solver.PackagesAssertions) {
	a.SourceAssertion = as
}

func (a *PackageArtifact) GetDependencies() []Artifact {
	return a.Dependencies
}

func (a *PackageArtifact) SetDependencies(d []Artifact) {
	a.Dependencies = d
}

func (a *PackageArtifact) GetPath() string {
	return a.Path
}

func (a *PackageArtifact) SetPath(p string) {
	a.Path = p
}

type CopyJob struct {
	Src, Dst string
	Artifact string
}

func worker(i int, wg *sync.WaitGroup, s <-chan CopyJob) {
	defer wg.Done()

	for job := range s {
		Info("#"+strconv.Itoa(i), "copying", job.Src, "to", job.Dst)
		if dir, err := helpers.IsDirectory(job.Src); err == nil && dir {
			err = helpers.CopyDir(job.Src, job.Dst)
			if err != nil {
				Fatal("Error copying dir", job, err)
			}
			continue
		}

		if !helpers.Exists(job.Dst) {
			if err := helpers.CopyFile(job.Src, job.Dst); err != nil {
				Fatal("Error copying", job, err)
			}
		}
	}
}

// ExtractArtifactFromDelta extracts deltas from ArtifactLayer from an image in tar format
func ExtractArtifactFromDelta(src, dst string, layers []ArtifactLayer, concurrency int, keepPerms bool, includes []string) (Artifact, error) {

	archive, err := ioutil.TempDir(os.TempDir(), "archive")
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating tempdir for archive")
	}
	defer os.RemoveAll(archive) // clean up

	if strings.HasSuffix(src, ".tar") {
		rootfs, err := ioutil.TempDir(os.TempDir(), "rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
		defer os.RemoveAll(rootfs) // clean up
		err = helpers.Untar(src, rootfs, keepPerms)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while unpacking rootfs")
		}
		src = rootfs
	}

	toCopy := make(chan CopyJob)

	var wg = new(sync.WaitGroup)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i, wg, toCopy)
	}

	// Handle includes in spec. If specified they filter what gets in the package
	if len(includes) > 0 {
		var includeRegexp []*regexp.Regexp
		for _, i := range includes {
			r, e := regexp.Compile(i)
			if e != nil {
				Warning("Failed compiling regex:", e)
				continue
			}
			includeRegexp = append(includeRegexp, r)
		}
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
		ADDS:
			for _, a := range l.Diffs.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) {
						toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
						continue ADDS
					}
				}
			}
		}
	} else {
		// Otherwise just grab all
		for _, l := range layers {
			// Consider d.Additions (and d.Changes? - warn at least) only
			for _, a := range l.Diffs.Additions {
				toCopy <- CopyJob{Src: filepath.Join(src, a.Name), Dst: filepath.Join(archive, a.Name), Artifact: a.Name}
			}
		}
	}

	close(toCopy)
	wg.Wait()

	err = helpers.Tar(archive, dst)
	if err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}
	return NewPackageArtifact(dst), nil
}
