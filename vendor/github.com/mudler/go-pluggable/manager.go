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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Manager describes a set of Plugins and
// a set of Event types which are subscribed to a message bus
type Manager struct {
	Plugins []Plugin
	Events  []EventType
	Bus     *Bus
}

// NewManager returns a manager instance with a new bus and
func NewManager(events []EventType) *Manager {
	return &Manager{
		Events: events,
		Bus:    NewBus(),
	}
}

// Register subscribes the plugin to its internal bus
func (m *Manager) Register() *Manager {
	m.Subscribe(m.Bus)
	return m
}

// Publish is a wrapper around NewEvent and the Manager internal Bus publishing system
func (m *Manager) Publish(event EventType, obj interface{}) (*Manager, error) {
	ev, err := NewEvent(event, obj)
	if err == nil && ev != nil {
		m.Bus.Publish(ev)
	}
	return m, err
}

// Subscribe subscribes the plugin to the events in the given bus
func (m *Manager) Subscribe(b *Bus) *Manager {
	for _, p := range m.Plugins {
		for _, e := range m.Events {
			b.Listen(e, b.propagateEvent(p))
		}
	}
	return m
}

func relativeToCwd(p string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "failed getting current work directory")
	}

	return filepath.Join(cwd, p), nil
}

// ListenAll Binds a callback to all plugins event
func (m *Manager) ListenAll(event EventType, listener interface{}) {
	for _, p := range m.Plugins {
		m.Bus.Listen(EventType(p.Name), listener)
	}
}

// Autoload automatically loads plugins binaries prefixed by 'prefix' in the current path
// optionally takes a list of paths to look also into
func (m *Manager) Autoload(prefix string, extensionpath ...string) *Manager {
	projPrefix := fmt.Sprintf("%s-", prefix)
	paths := strings.Split(os.Getenv("PATH"), ":")

	for _, path := range extensionpath {
		if filepath.IsAbs(path) {
			paths = append(paths, path)
			continue
		}

		rel, err := relativeToCwd(path)
		if err != nil {
			continue
		}
		paths = append(paths, rel)
	}

	for _, p := range paths {
		matches, err := filepath.Glob(filepath.Join(p, fmt.Sprintf("%s*", projPrefix)))
		if err != nil {
			continue
		}
		for _, ma := range matches {
			short := strings.TrimPrefix(filepath.Base(ma), projPrefix)
			m.Plugins = append(m.Plugins, Plugin{Name: short, Executable: ma})
		}
	}
	return m
}

// Load finds the binaries given as parameter (without path) and scan the system $PATH to retrieve those automatically
func (m *Manager) Load(extname ...string) *Manager {
	paths := strings.Split(os.Getenv("PATH"), ":")

	for _, p := range paths {
		for _, n := range extname {
			path := filepath.Join(p, n)
			_, err := os.Lstat(path)
			if err != nil {
				continue
			}
			m.Plugins = append(m.Plugins, Plugin{Name: n, Executable: path})
		}
	}
	return m
}
