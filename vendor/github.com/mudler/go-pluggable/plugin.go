// Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
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

package pluggable

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

// Plugin describes binaries to be hooked on events, with common js input, and common js output
type Plugin struct {
	Name       string
	Executable string
}

// Run runs the Event on the plugin, and returns an EventResponse
func (p Plugin) Run(e Event) (EventResponse, error) {
	r := EventResponse{}
	k, err := e.JSON()
	if err != nil {
		return r, errors.Wrap(err, "while marshalling event")
	}
	cmd := exec.Command(p.Executable, string(e.Name), k)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		r.Error = err.Error()
		return r, errors.Wrap(err, "while executing plugin")
	}

	if err := json.Unmarshal(out, &r); err != nil {
		r.Error = err.Error()
		return r, errors.Wrap(err, "while unmarshalling response")
	}
	return r, nil
}
