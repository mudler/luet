// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

	"github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/pkg/errors"
)

const (
	ImgBackend      = "img"
	DockerBackend   = "docker"
	Dockerv2Backend = "dockerv2"
)

func imageAvailable(image string) bool {
	_, err := crane.Digest(image)
	return err == nil
}

type Options struct {
	ImageName      string
	SourcePath     string
	DockerFileName string
	Destination    string
	Context        string
	BackendArgs    []string
	PackageDir     string
}

func runCommand(cmd *exec.Cmd) error {
	output := ""
	buffered := !config.LuetCfg.GetGeneral().ShowBuildOutput
	writer := NewBackendWriter(buffered)

	cmd.Stdout = writer
	cmd.Stderr = writer

	if buffered {
		Spinner(22)
		defer SpinnerStop()
	}

	err := cmd.Start()
	if err != nil {
		return errors.Wrap(err, "Failed starting command")
	}

	err = cmd.Wait()
	if err != nil {
		output = writer.GetCombinedOutput()
		return errors.Wrapf(err, "Failed running command: %s", output)
	}

	return nil
}

func genBuildCommand(opts Options) []string {
	context := opts.Context

	if context == "" {
		context = "."
	}
	buildarg := append(opts.BackendArgs, "-f", opts.DockerFileName, "-t", opts.ImageName, context)
	return append([]string{"build"}, buildarg...)
}
