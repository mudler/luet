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

import (
	"github.com/chuckpreslar/emission"
	"github.com/codegangsta/inject"
)

type Bus struct {
	inject.Injector
	emission.Emitter
}

func NewBus() *Bus {
	return &Bus{
		Injector: inject.New(),
		Emitter:  *emission.NewEmitter(),
	}
}

// On Binds a callback to an event, mapping the arguments on a global level
func (a *Bus) Listen(event EventType, listener interface{}) *Bus {
	a.On(string(event), func() { a.Invoke(listener) })
	return a
}

// Emit Emits an event, it does accept only the event as argument, since
// the callback will have access to the service mapped by the injector
func (a *Bus) Publish(t EventType, e *Event) *Bus {
	a.Map(e)
	a.Emit(string(t))
	return a
}

// Once Binds a callback to an event, mapping the arguments on a global level
// It is fired only once.
func (a *Bus) OnlyOnce(event EventType, listener interface{}) *Bus {
	a.Once(string(event), func() { a.Invoke(listener) })
	return a
}

// EmitSync Emits an event in a syncronized manner,
// it does accept only the event as argument, since
// the callback will have access to the service mapped by the injector
func (a *Bus) EmitSync(event interface{}) *Bus {
	a.EmitSync(event)
	return a
}

func (b *Bus) PropagateEvent(p Plugin) func(e *Event) {
	return func(e *Event) {
		resp, _ := p.Run(*e)
		b.Map(&resp)
		b.Emit(p.Name)
	}
}
