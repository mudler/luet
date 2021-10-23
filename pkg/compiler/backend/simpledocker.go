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

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/mudler/luet/pkg/api/core/types"
	bus "github.com/mudler/luet/pkg/bus"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pkg/errors"
)

type SimpleDocker struct {
	ctx *types.Context
}

func NewSimpleDockerBackend(ctx *types.Context) *SimpleDocker {
	return &SimpleDocker{ctx: ctx}
}

// TODO: Missing still: labels, and build args expansion
func (s *SimpleDocker) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)
	s.ctx.Info(":whale2: Building image " + name)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = opts.SourcePath
	err := runCommand(s.ctx, cmd)
	if err != nil {
		return err
	}

	s.ctx.Success(":whale: Building image " + name + " done")

	bus.Manager.Publish(bus.EventImagePostBuild, opts)

	return nil
}

func (s *SimpleDocker) CopyImage(src, dst string) error {
	s.ctx.Debug(":whale: Tagging image:", src, "->", dst)
	cmd := exec.Command("docker", "tag", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}
	s.ctx.Success(":whale: Tagged image:", src, "->", dst)
	return nil
}

func (s *SimpleDocker) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	buildarg := []string{"pull", name}
	s.ctx.Debug(":whale: Downloading image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pulling image: "+string(out))
	}

	s.ctx.Success(":whale: Downloaded image:", name)
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}

func (s *SimpleDocker) ImageExists(imagename string) bool {
	buildarg := []string{"inspect", "--type=image", imagename}
	s.ctx.Debug(":whale: Checking existance of docker image: " + imagename)
	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.ctx.Debug("Image not present")
		s.ctx.Debug(string(out))
		return false
	}
	return true
}

func (*SimpleDocker) ImageAvailable(imagename string) bool {
	return imageAvailable(imagename)
}

func (s *SimpleDocker) RemoveImage(opts Options) error {
	name := opts.ImageName
	buildarg := []string{"rmi", name}
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}
	s.ctx.Success(":whale: Removed image:", name)
	//Info(string(out))
	return nil
}

func (s *SimpleDocker) Push(opts Options) error {
	name := opts.ImageName
	pusharg := []string{"push", name}
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("docker", pusharg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}
	s.ctx.Success(":whale: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	//Info(string(out))
	return nil
}

func (s *SimpleDocker) ImageReference(a string) (v1.Image, error) {
	ref, err := name.ParseReference(a)
	if err != nil {
		return nil, err
	}

	img, err := daemon.Image(ref)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (s *SimpleDocker) ImageDefinitionToTar(opts Options) error {
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

func (s *SimpleDocker) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination

	buildarg := []string{"save", name, "-o", path}
	s.ctx.Debug(":whale: Saving image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	s.ctx.Debug(":whale: Exported image:", name)
	return nil
}

type ManifestEntry struct {
	Layers []string `json:"Layers"`
}
