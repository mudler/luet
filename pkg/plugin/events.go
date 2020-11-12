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

package plugin

import "encoding/json"

var (
	PackageInstalled EventType = "package.install"
)

type EventType string

type Event struct {
	Name EventType `json:"name"`
	Data string    `json:"data"`
}

type EventResponse struct {
	State string `json:"state"`
	Data  string `json:"data"`
	Error string `json:"error"`
}

func (e Event) JSON() (string, error) {
	dat, err := json.Marshal(e)
	return string(dat), err
}

func (r EventResponse) Unmarshal(i interface{}) error {
	return json.Unmarshal([]byte(r.Data), i)
}

func (r EventResponse) Errored() bool {
	return len(r.Error) != 0
}

func NewEvent(name EventType, obj interface{}) (*Event, error) {
	dat, err := json.Marshal(obj)
	return &Event{Name: name, Data: string(dat)}, err
}
