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
	"path"
	"path/filepath"
	"regexp"

	//"strconv"
	"strings"
	"sync"

	. "github.com/mudler/luet/pkg/logger"
	"github.com/mudler/luet/pkg/solver"
	yaml "gopkg.in/yaml.v2"

	"github.com/mudler/luet/pkg/helpers"
	"github.com/pkg/errors"
)

type ArtifactIndex []Artifact

func (i ArtifactIndex) CleanPath() ArtifactIndex {
	newIndex := ArtifactIndex{}
	for _, n := range i {
		art := n.(*PackageArtifact)
		newIndex = append(newIndex, &PackageArtifact{Path: path.Base(n.GetPath()), SourceAssertion: art.SourceAssertion, CompileSpec: art.CompileSpec, Dependencies: art.Dependencies})
	}
	return newIndex
	//Update if exists, otherwise just create
}

//  When compiling, we write also a fingerprint.metadata.yaml file with PackageArtifact. In this way we can have another command to create the repository
// which will consist in just of an repository.yaml which is just the repository structure with the list of package artifact.
// In this way a generic client can fetch the packages and, after unpacking the tree, performing queries to install packages.
type PackageArtifact struct {
	Path         string               `json:"path"`
	Dependencies []*PackageArtifact   `json:"dependencies"`
	CompileSpec  *LuetCompilationSpec `json:"compilationspec"`

	SourceAssertion solver.PackagesAssertions `json:"-"`
}

func NewPackageArtifact(path string) Artifact {
	return &PackageArtifact{Path: path, Dependencies: []*PackageArtifact{}}
}

func NewPackageArtifactFromYaml(data []byte) (Artifact, error) {
	p := &PackageArtifact{}
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return p, err
	}

	return p, err
}

func (a *PackageArtifact) WriteYaml(dst string) error {
	//p := a.CompileSpec.GetPackage().GetPath()

	//a.CompileSpec.GetPackage().SetPath("")
	//	for _, ass := range a.CompileSpec.GetSourceAssertion() {
	//		ass.Package.SetPath("")
	//	}
	data, err := yaml.Marshal(a)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	mangle, err := NewPackageArtifactFromYaml(data)
	if err != nil {
		return errors.Wrap(err, "Generated invalid artifact")
	}
	//p := a.CompileSpec.GetPackage().GetPath()

	mangle.GetCompileSpec().GetPackage().SetPath("")
	for _, ass := range mangle.GetCompileSpec().GetSourceAssertion() {
		ass.Package.SetPath("")
	}

	data, err = yaml.Marshal(mangle)
	if err != nil {
		return errors.Wrap(err, "While marshalling for PackageArtifact YAML")
	}

	err = ioutil.WriteFile(filepath.Join(dst, a.GetCompileSpec().GetPackage().GetFingerPrint()+".metadata.yaml"), data, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "While writing PackageArtifact YAML")
	}
	//a.CompileSpec.GetPackage().SetPath(p)

	return nil
}

func (a *PackageArtifact) GetSourceAssertion() solver.PackagesAssertions {
	return a.SourceAssertion
}

func (a *PackageArtifact) SetCompileSpec(as CompilationSpec) {
	a.CompileSpec = as.(*LuetCompilationSpec)
}

func (a *PackageArtifact) GetCompileSpec() CompilationSpec {
	return a.CompileSpec
}

func (a *PackageArtifact) SetSourceAssertion(as solver.PackagesAssertions) {
	a.SourceAssertion = as
}

func (a *PackageArtifact) GetDependencies() []Artifact {
	ret := []Artifact{}
	for _, d := range a.Dependencies {
		ret = append(ret, d)
	}
	return ret
}

func (a *PackageArtifact) SetDependencies(d []Artifact) {
	ret := []*PackageArtifact{}
	for _, dd := range d {
		ret = append(ret, dd.(*PackageArtifact))
	}
	a.Dependencies = ret
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
		//Info("#"+strconv.Itoa(i), "copying", job.Src, "to", job.Dst)
		// if dir, err := helpers.IsDirectory(job.Src); err == nil && dir {
		// 	err = helpers.CopyDir(job.Src, job.Dst)
		// 	if err != nil {
		// 		Warning("Error copying dir", job, err)
		// 	}
		// 	continue
		// }

		if !helpers.Exists(job.Dst) {
			if err := helpers.CopyFile(job.Src, job.Dst); err != nil {
				Warning("Error copying", job, err)
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
