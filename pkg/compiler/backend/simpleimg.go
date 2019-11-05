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

type SimpleImg struct{}

func NewSimpleImgBackend() compiler.CompilerBackend {
	return &SimpleImg{}
}

// TODO: Missing still: labels, and build args expansion
func (*SimpleImg) BuildImage(name, path, dockerfileName string) error {
	buildarg := "img build -t " + name + " " + path + " -f " + dockerfileName
	Spinner(22)
	Debug("Building image "+name+" - running img with: ", buildarg)
	out, err := exec.Command(buildarg).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+out)
	}
	SpinnerStop()
	Info(out)
	return nil
}

func (*SimpleImg) ExportImage(name, path string) error {
	buildarg := "img save " + name + " -o " + path
	Spinner(22)
	Debug("Saving image "+name+" - running img with: ", buildarg)
	out, err := exec.Command(buildarg).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed building image: "+out)
	}
	SpinnerStop()
	Info(out)
	return nil

}

// TODO: Use container-diff (https://github.com/GoogleContainerTools/container-diff) for checking out layer diffs
