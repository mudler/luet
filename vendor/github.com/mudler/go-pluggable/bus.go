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
	"sync"

	"github.com/chuckpreslar/emission"
	"github.com/codegangsta/inject"
)

// Bus represent the bus event system
type Bus struct {
	inject.Injector
	emission.Emitter
	sync.Mutex
}

// NewBus returns a new Bus instance
func NewBus() *Bus {
	return &Bus{
		Injector: inject.New(),
		Emitter:  *emission.NewEmitter(),
	}
}

// Listen Binds a callback to an event, mapping the arguments on a global level
func (a *Bus) Listen(event EventType, listener interface{}) *Bus {
	a.Lock()
	defer a.Unlock()
	a.On(string(event), func() { a.Invoke(listener) })
	return a
}

// Publish publishes an event, it does accept only the event as argument, since
// the callback will have access to the service mapped by the injector
func (a *Bus) Publish(e *Event) *Bus {
	a.Lock()
	defer a.Unlock()
	a.Map(e)
	a.Emit(string(e.Name))
	return a
}

// OnlyOnce Binds a callback to an event, mapping the arguments on a global level
// It is fired only once.
func (a *Bus) OnlyOnce(event EventType, listener interface{}) *Bus {
	a.Lock()
	defer a.Unlock()
	a.Once(string(event), func() { a.Invoke(listener) })
	return a
}

func (a *Bus) propagateEvent(p Plugin) func(e *Event) {
	return func(e *Event) {
		resp, _ := p.Run(*e)
		a.Map(&resp)
		a.Emit(p.Name)
	}
}
