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

package backend

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	capi "github.com/mudler/docker-companion/api"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/pkg/errors"
)

type SimpleDocker struct{}

func NewSimpleDockerBackend() compiler.CompilerBackend {
	return &SimpleDocker{}
}

// TODO: Missing still: labels, and build args expansion
func (*SimpleDocker) BuildImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	path := opts.SourcePath
	dockerfileName := opts.DockerFileName
	buildarg := []string{"build", "-f", dockerfileName, "-t", name, "."}

	Debug(":whale2: Building image " + name)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	Info(":whale: Building image " + name + " done")

	//Info(string(out))
	return nil
}

func (*SimpleDocker) CopyImage(src, dst string) error {
	Debug(":whale: Tagging image:", src, "->", dst)
	cmd := exec.Command("docker", "tag", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}
	Info(":whale: Tagged image:", src, "->", dst)
	return nil
}

func (*SimpleDocker) DownloadImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	buildarg := []string{"pull", name}
	Debug(":whale: Downloading image " + name)
	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	Info(":whale: Downloaded image:", name)
	return nil
}

func (*SimpleDocker) RemoveImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	buildarg := []string{"rmi", name}
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}
	Info(":whale: Removed image:", name)
	//Info(string(out))
	return nil
}

func (s *SimpleDocker) ImageDefinitionToTar(opts compiler.CompilerBackendOptions) error {
	if err := s.BuildImage(opts); err != nil {
		return errors.Wrap(err, "Failed building image")
	}
	if err := s.ExportImage(opts); err != nil {
		return errors.Wrap(err, "Failed exporting image")
	}
	if err := s.RemoveImage(opts); err != nil {
		return errors.Wrap(err, "Failed removing image")
	}
	return nil
}

func (*SimpleDocker) ExportImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	path := opts.Destination

	buildarg := []string{"save", name, "-o", path}
	Debug(":whale: Saving image " + name)
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	Info(":whale: Exported image:", name)
	return nil
}

type ManifestEntry struct {
	Layers []string `json:"Layers"`
}

func (*SimpleDocker) ExtractRootfs(opts compiler.CompilerBackendOptions, keepPerms bool) error {
	src := opts.SourcePath
	dst := opts.Destination

	rootfs, err := ioutil.TempDir(dst, "tmprootfs")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(rootfs) // clean up

	// TODO: Following as option if archive as output?
	// archive, err := ioutil.TempDir(os.TempDir(), "archive")
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	// }
	// defer os.RemoveAll(archive) // clean up

	err = helpers.Untar(src, rootfs, keepPerms)
	if err != nil {
		return errors.Wrap(err, "Error met while unpacking rootfs")
	}

	manifest, err := helpers.Read(filepath.Join(rootfs, "manifest.json"))
	if err != nil {
		return errors.Wrap(err, "Error met while reading image manifest")
	}

	// Unpack all layers
	var manifestData []ManifestEntry

	if err := json.Unmarshal([]byte(manifest), &manifestData); err != nil {
		return errors.Wrap(err, "Error met while unmarshalling manifest")
	}

	layers_sha := []string{}

	for _, data := range manifestData {
		for _, l := range data.Layers {
			layers_sha = append(layers_sha, strings.Replace(l, "/layer.tar", "", -1))
		}
	}

	export, err := capi.CreateExport(rootfs)
	if err != nil {
		return err
	}

	err = export.UnPackLayers(layers_sha, dst, "")
	if err != nil {
		return err
	}

	// err = helpers.Tar(archive, dst)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Error met while creating package archive")
	// }

	return nil
}

// 	container-diff diff daemon://luet/base alpine --type=file -j
// [
//   {
//     "Image1": "luet/base",
//     "Image2": "alpine",
//     "DiffType": "File",
//     "Diff": {
//       "Adds": null,
//       "Dels": [
//         {
//           "Name": "/luetbuild",
//           "Size": 5830706
//         },
//         {
//           "Name": "/luetbuild/Dockerfile",
//           "Size": 50
//         },
//         {
//           "Name": "/luetbuild/output1",
//           "Size": 5830656
//         }
//       ],
//       "Mods": null
//     }
//   }
// ]
// Changes uses container-diff (https://github.com/GoogleContainerTools/container-diff) for retrieving out layer diffs
func (*SimpleDocker) Changes(fromImage, toImage string) ([]compiler.ArtifactLayer, error) {
	tmpdiffs, err := ioutil.TempDir(os.TempDir(), "tmpdiffs")
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(tmpdiffs) // clean up

	diffargs := []string{"diff", fromImage, toImage, "--type=file", "-j", "-n", "-c", tmpdiffs}
	out, err := exec.Command("container-diff", diffargs...).CombinedOutput()
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Failed Resolving layer diffs: "+string(out))
	}

	var diffs []compiler.ArtifactLayer

	err = json.Unmarshal(out, &diffs)
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Failed unmarshalling json response: "+string(out))
	}
	return diffs, nil
}
