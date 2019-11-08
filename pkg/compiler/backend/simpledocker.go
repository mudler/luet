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
	Debug("Building image "+name+" - running docker with: ", buildarg)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	SpinnerStop()
	Info(string(out))
	return nil
}

func (*SimpleDocker) RemoveImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	buildarg := []string{"rmi", name}
	Spinner(22)
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}
	SpinnerStop()
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
	Debug("Saving image "+name+" - running docker with: ", buildarg)
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}
	SpinnerStop()
	Info(string(out))
	return nil
}

// TODO: Use container-diff (https://github.com/GoogleContainerTools/container-diff) for checking out layer diffs
