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
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bus "github.com/mudler/luet/pkg/bus"

	capi "github.com/mudler/docker-companion/api"

	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/pkg/errors"
)

type SimpleDocker struct{}

func NewSimpleDockerBackend() *SimpleDocker {
	return &SimpleDocker{}
}

// TODO: Missing still: labels, and build args expansion
func (*SimpleDocker) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)
	Info(":whale2: Building image " + name)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = opts.SourcePath
	err := runCommand(cmd)
	if err != nil {
		return err
	}

	Info(":whale: Building image " + name + " done")

	bus.Manager.Publish(bus.EventImagePostBuild, opts)

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

func (*SimpleDocker) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	buildarg := []string{"pull", name}
	Debug(":whale: Downloading image " + name)

	Spinner(22)
	defer SpinnerStop()

	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pulling image: "+string(out))
	}

	Info(":whale: Downloaded image:", name)
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}

func (*SimpleDocker) ImageExists(imagename string) bool {
	buildarg := []string{"inspect", "--type=image", imagename}
	Debug(":whale: Checking existance of docker image: " + imagename)
	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		Debug("Image not present")
		Debug(string(out))
		return false
	}
	return true
}

func (*SimpleDocker) ImageAvailable(imagename string) bool {
	return imageAvailable(imagename)
}

func (*SimpleDocker) RemoveImage(opts Options) error {
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

func (*SimpleDocker) Push(opts Options) error {
	name := opts.ImageName
	pusharg := []string{"push", name}
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	Spinner(22)
	defer SpinnerStop()

	out, err := exec.Command("docker", pusharg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}
	Info(":whale: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	//Info(string(out))
	return nil
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

func (*SimpleDocker) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination

	buildarg := []string{"save", name, "-o", path}
	Debug(":whale: Saving image " + name)

	Spinner(22)
	defer SpinnerStop()

	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	Debug(":whale: Exported image:", name)
	return nil
}

type ManifestEntry struct {
	Layers []string `json:"Layers"`
}

func (b *SimpleDocker) ExtractRootfs(opts Options, keepPerms bool) error {
	name := opts.ImageName
	dst := opts.Destination

	if !b.ImageExists(name) {
		if err := b.DownloadImage(opts); err != nil {
			return errors.Wrap(err, "failed pulling image "+name+" during extraction")
		}
	}

	tempexport, err := ioutil.TempDir(dst, "tmprootfs")
	if err != nil {
		return errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(tempexport) // clean up

	imageExport := filepath.Join(tempexport, "image.tar")

	Spinner(22)
	defer SpinnerStop()

	if err := b.ExportImage(Options{ImageName: name, Destination: imageExport}); err != nil {
		return errors.Wrap(err, "failed while extracting rootfs for "+name)
	}

	SpinnerStop()

	src := imageExport

	if src == "" && opts.ImageName != "" {
		tempUnpack, err := ioutil.TempDir(dst, "tempUnpack")
		if err != nil {
			return errors.Wrap(err, "Error met while creating tempdir for rootfs")
		}
		defer os.RemoveAll(tempUnpack) // clean up
		imageExport := filepath.Join(tempUnpack, "image.tar")
		if err := b.ExportImage(Options{ImageName: opts.ImageName, Destination: imageExport}); err != nil {
			return errors.Wrap(err, "while exporting image before extraction")
		}
		src = imageExport
	}

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

	manifest, err := fileHelper.Read(filepath.Join(rootfs, "manifest.json"))
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
			if strings.Contains(l, "layer.tar") {
				layers_sha = append(layers_sha, strings.Replace(l, "/layer.tar", "", -1))
			}
		}
	}
	// TODO: Drop capi in favor of the img approach already used in pkg/installer/repository
	export, err := capi.CreateExport(rootfs)
	if err != nil {
		return err
	}

	err = export.UnPackLayers(layers_sha, dst, "containerd")
	if err != nil {
		return err
	}

	// err = helpers.Tar(archive, dst)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Error met while creating package archive")
	// }

	return nil
}
