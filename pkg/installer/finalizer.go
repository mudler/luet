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

package installer

import (
	"os"
	"os/exec"

	"github.com/ghodss/yaml"
	box "github.com/mudler/luet/pkg/box"
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/pkg/errors"
)

type LuetFinalizer struct {
	Shell     []string `json:"shell"`
	Install   []string `json:"install"`
	Uninstall []string `json:"uninstall"` // TODO: Where to store?
}

func (f *LuetFinalizer) RunInstall(s *System) error {
	var cmd string
	var args []string
	if len(f.Shell) == 0 {
		// Default to sh otherwise
		cmd = "sh"
		args = []string{"-c"}
	} else {
		cmd = f.Shell[0]
		if len(f.Shell) > 1 {
			args = f.Shell[1:]
		}
	}

	for _, c := range f.Install {
		toRun := append(args, c)
		Info(":shell: Executing finalizer on ", s.Target, cmd, toRun)
		if s.Target == string(os.PathSeparator) {
			cmd := exec.Command(cmd, toRun...)
			cmd.Env = LuetCfg.GetFinalizerEnvs()
			stdoutStderr, err := cmd.CombinedOutput()
			if err != nil {
				return errors.Wrap(err, "Failed running command: "+string(stdoutStderr))
			}
			Info(string(stdoutStderr))
		} else {
			b := box.NewBox(cmd, toRun, []string{}, LuetCfg.GetFinalizerEnvs(), s.Target, false, true, true)
			err := b.Run()
			if err != nil {
				return errors.Wrap(err, "Failed running command ")
			}
		}
	}
	return nil
}

// TODO: We don't store uninstall finalizers ?!
func (f *LuetFinalizer) RunUnInstall() error {
	for _, c := range f.Uninstall {
		Debug("finalizer:", "sh", "-c", c)
		cmd := exec.Command("sh", "-c", c)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "Failed running command: "+string(stdoutStderr))
		}
		Info(string(stdoutStderr))
	}
	return nil
}

func NewLuetFinalizerFromYaml(data []byte) (*LuetFinalizer, error) {
	var p LuetFinalizer
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return &p, err
	}
	return &p, err
}
