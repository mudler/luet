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
	"os"
	"os/exec"

	"github.com/mudler/luet/pkg/compiler"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/pkg/errors"
)

type SimpleImg struct{}

func NewSimpleImgBackend() compiler.CompilerBackend {
	return &SimpleImg{}
}

// TODO: Missing still: labels, and build args expansion
func (*SimpleImg) BuildImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	path := opts.SourcePath
	dockerfileName := opts.DockerFileName

	buildarg := []string{"build", "-f", dockerfileName, "-t", name, "."}
	Spinner(22)
	defer SpinnerStop()
	Debug(":tea: Building image " + name)
	cmd := exec.Command("img", buildarg...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()

	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	Info(":tea: Building image " + name + " done")
	return nil
}

func (*SimpleImg) RemoveImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	buildarg := []string{"rm", name}
	Spinner(22)
	defer SpinnerStop()
	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}

	Info(":tea: Image " + name + " removed")
	return nil
}

func (*SimpleImg) DownloadImage(opts compiler.CompilerBackendOptions) error {

	name := opts.ImageName
	buildarg := []string{"pull", name}

	Debug(":tea: Downloading image " + name)
	cmd := exec.Command("img", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}

	Info(":tea: Image " + name + " downloaded")

	return nil
}
func (*SimpleImg) CopyImage(src, dst string) error {
	Spinner(22)
	defer SpinnerStop()

	Debug(":tea: Tagging image", src, dst)
	cmd := exec.Command("img", "tag", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}
	Info(":tea: Image " + dst + " tagged")

	return nil
}

func (*SimpleImg) ImageExists(imagename string) bool {
	// NOOP: not implemented
	// TODO: Since img doesn't have an inspect command,
	// we need to parse the ls output manually
	return false
}

func (s *SimpleImg) ImageDefinitionToTar(opts compiler.CompilerBackendOptions) error {
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

func (*SimpleImg) ExportImage(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	path := opts.Destination
	buildarg := []string{"save", "-o", path, name}
	Debug(":tea: Saving image " + name)
	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+string(out))
	}
	Info(":tea: Image " + name + " saved")
	return nil
}

// TODO: Dup in docker, refactor common code in helpers for shared parts
func (*SimpleImg) ExtractRootfs(opts compiler.CompilerBackendOptions, keepPerms bool) error {
	name := opts.ImageName
	path := opts.Destination

	os.RemoveAll(path)
	buildarg := []string{"unpack", "-o", path, name}
	Debug(":tea: Extracting image " + name)
	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed extracting image: "+string(out))
	}
	Info(":tea: Image " + name + " extracted")
	return nil
	//return NewSimpleDockerBackend().ExtractRootfs(opts, keepPerms)
}

// TODO: Use container-diff (https://github.com/GoogleContainerTools/container-diff) for checking out layer diffs
// Changes uses container-diff (https://github.com/GoogleContainerTools/container-diff) for retrieving out layer diffs
func (i *SimpleImg) Changes(fromImage, toImage compiler.CompilerBackendOptions) ([]compiler.ArtifactLayer, error) {
	return GenerateChanges(i, fromImage, toImage)
}

func (*SimpleImg) Push(opts compiler.CompilerBackendOptions) error {
	name := opts.ImageName
	pusharg := []string{"push", name}
	out, err := exec.Command("img", pusharg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}
	Info(":tea: Pushed image:", name)
	//Info(string(out))
	return nil
}
