// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// dockerArchive builds the containers/image transport string used to move
// images between buildah and go-containerregistry. It must be docker-archive
// rather than oci-archive: go-containerregistry's tarball reader only accepts
// the former.
func dockerArchive(path string) string {
	return "docker-archive:" + path
}

type SimpleBuildah struct {
	ctx types.Context
}

func NewSimpleBuildahBackend(ctx types.Context) *SimpleBuildah {
	return &SimpleBuildah{ctx: ctx}
}

func (s *SimpleBuildah) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)
	s.ctx.Info(":tea: Building image " + name)

	cmd := exec.Command("buildah", buildarg...)
	cmd.Dir = opts.SourcePath
	if err := runCommand(s.ctx, cmd); err != nil {
		return err
	}

	s.ctx.Success(":tea: Building image " + name + " done")
	bus.Manager.Publish(bus.EventImagePostBuild, opts)

	return nil
}

func (s *SimpleBuildah) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination

	s.ctx.Debug(":tea: Saving image " + name)
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	// buildah's docker-archive writer refuses a non-empty existing path
	// ("docker-archive doesn't support modifying existing images"), whereas
	// docker save -o truncates. Remove first so both backends behave alike.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "Failed clearing destination archive")
	}

	out, err := exec.Command("buildah", "push", name, dockerArchive(path)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	s.ctx.Success(":tea: Image " + name + " saved")
	return nil
}

// ImageReference returns a go-containerregistry handle on the image. buildah
// has no daemon to query, so ondisk is ignored and the image is always routed
// through a docker-archive on disk.
func (s *SimpleBuildah) ImageReference(a string, ondisk bool) (v1.Image, error) {
	f, err := s.ctx.TempFile("snapshot")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Remove before pushing: buildah's docker-archive writer only tolerates a
	// zero-byte existing path, which is an implementation detail rather than a
	// documented contract.
	//
	// The archive is deliberately NOT removed on return. crane.Load is lazy:
	// the returned v1.Image reads layer bytes from this path on demand, so
	// deleting it here yields a handle whose Layers() succeeds (manifest data
	// is eager) while every content read fails with ENOENT -- which breaks the
	// flatten/diff hot path. SimpleDocker leaves its snapshot behind for the
	// same reason; both rely on the context garbage collector, which owns this
	// directory and drops it wholesale on Clean().
	os.Remove(f.Name())

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "push", a, dockerArchive(f.Name())).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed saving image: "+string(out))
	}

	img, err := crane.Load(f.Name())
	if err != nil {
		return nil, err
	}

	return img, nil
}

// LoadImage imports a docker-archive produced by ExportImage. The img backend
// could not do this at all, which is why create-repo --type docker did not
// work on the daemonless path.
func (s *SimpleBuildah) LoadImage(path string) error {
	s.ctx.Debug(":tea: Loading image:", path)

	out, err := exec.Command("buildah", "pull", dockerArchive(path)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed loading image: "+string(out))
	}

	s.ctx.Success(":tea: Loaded image:", path)
	return nil
}

func (s *SimpleBuildah) RemoveImage(opts Options) error {
	name := opts.ImageName

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "rmi", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}

	s.ctx.Success(":tea: Removed image:", name)
	return nil
}

func (s *SimpleBuildah) CopyImage(src, dst string) error {
	s.ctx.Debug(":tea: Tagging image:", src, "->", dst)

	out, err := exec.Command("buildah", "tag", src, dst).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}

	s.ctx.Success(":tea: Tagged image:", src, "->", dst)
	return nil
}

func (s *SimpleBuildah) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	s.ctx.Debug(":tea: Downloading image " + name)
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "pull", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pulling image: "+string(out))
	}

	s.ctx.Success(":tea: Downloaded image:", name)
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}

func (s *SimpleBuildah) Push(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "push", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}

	s.ctx.Success(":tea: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	return nil
}

func (*SimpleBuildah) ImageAvailable(imagename string) bool {
	return image.Available(imagename)
}

// ImageExists reports whether the image is present locally. It uses buildah
// inspect rather than matching against the output of buildah images: the img
// backend used strings.Contains over `img ls`, which false-positives on any
// name containing the queried name as a substring.
func (s *SimpleBuildah) ImageExists(imagename string) bool {
	s.ctx.Debug(":tea: Checking existence of image: " + imagename)

	cmd := exec.Command("buildah", "inspect", "--type", "image", imagename)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.ctx.Debug("Image not present")
		s.ctx.Debug(string(out))
		return false
	}
	return true
}

func (s *SimpleBuildah) ImageDefinitionToTar(opts Options) error {
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

// Compile-time check that SimpleBuildah satisfies the backend contract.
// The interface lives in package compiler, so this is asserted there in
// backend.go's factory; this local assertion catches drift earlier.
var _ interface {
	BuildImage(Options) error
	ExportImage(Options) error
	LoadImage(string) error
	RemoveImage(Options) error
	ImageDefinitionToTar(Options) error
	CopyImage(string, string) error
	DownloadImage(Options) error
	Push(Options) error
	ImageAvailable(string) bool
	ImageReference(string, bool) (v1.Image, error)
	ImageExists(string) bool
} = (*SimpleBuildah)(nil)
