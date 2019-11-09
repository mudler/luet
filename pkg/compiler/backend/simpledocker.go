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
	"os/exec"

	"github.com/mudler/luet/pkg/compiler"
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
	Spinner(22)
	defer SpinnerStop()

	Debug("Building image "+name+" - running docker with: ", buildarg)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	Info(string(out))
	return nil
}

func (*SimpleDocker) RemoveImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	buildarg := []string{"rmi", name}
	Spinner(22)
	defer SpinnerStop()
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}
	Info(string(out))
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
	Spinner(22)
	defer SpinnerStop()
	Debug("Saving image "+name+" - running docker with: ", buildarg)
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	Info(string(out))
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
	diffargs := []string{"diff", fromImage, toImage, "--type=file", "-j"}
	Spinner(22)
	defer SpinnerStop()

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
