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
	"fmt"
	"os/exec"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/pkg/errors"
)

const (
	ImgBackend    = "img"
	DockerBackend = "docker"
)

func imageAvailable(image string) bool {
	_, err := crane.Digest(image)
	return err == nil
}

func NewBackend(s string) compiler.CompilerBackend {
	var compilerBackend compiler.CompilerBackend

	switch s {
	case ImgBackend:
		compilerBackend = NewSimpleImgBackend()
	case DockerBackend:
		compilerBackend = NewSimpleDockerBackend()
	}
	return compilerBackend
}

func runCommand(cmd *exec.Cmd) (string, error) {
	ans := ""
	writer := NewBackendWriter(!config.LuetCfg.GetGeneral().ShowBuildOutput)

	if config.LuetCfg.GetGeneral().ShowBuildOutput {
		// We have realtime output from command. Quiet spinner.
		SpinnerStop()
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Start()
	if err != nil {
		return "", errors.Wrap(err, "Failed starting build")
	}

	err = cmd.Wait()
	if err != nil {
		return "", errors.Wrap(err, "Failed waiting for building command")
	}

	res := cmd.ProcessState.ExitCode()
	if res != 0 {
		errMsg := fmt.Sprintf("Failed building image (exiting with %d)", res)
		if !config.LuetCfg.GetGeneral().ShowBuildOutput {
			errMsg = fmt.Sprintf("Failed building image (exiting with %d): %s",
				res, writer.GetCombinedOutput())
		}

		return "", errors.Wrap(err, errMsg)
	}

	if config.LuetCfg.GetGeneral().ShowBuildOutput {
		Spinner(22)
	} else {
		ans = writer.GetCombinedOutput()
	}

	return ans, nil
}
