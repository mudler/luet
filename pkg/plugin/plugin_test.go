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

package plugin_test

import (
	"io/ioutil"
	"os"

	. "github.com/mudler/luet/pkg/plugin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plugin", func() {
	Context("event subscription", func() {
		var pluginFile *os.File
		var err error
		var b *Bus
		var m *Manager

		BeforeEach(func() {
			pluginFile, err = ioutil.TempFile(os.TempDir(), "tests")
			Expect(err).Should(BeNil())
			defer os.Remove(pluginFile.Name()) // clean up
			b = NewBus()
			m = &Manager{}
		})

		It("gets the json event name", func() {
			d1 := []byte("#!/bin/bash\necho \"{ \\\"state\\\": \\\"$1\\\" }\"\n")
			err := ioutil.WriteFile(pluginFile.Name(), d1, 0550)
			Expect(err).Should(BeNil())

			m.Plugins = []Plugin{{Name: "test", Executable: pluginFile.Name()}}
			m.Events = []EventType{PackageInstalled}
			m.Subscribe(b)

			ev, err := NewEvent(PackageInstalled, map[string]string{"foo": "bar"})
			Expect(err).Should(BeNil())

			var received map[string]string
			var resp *EventResponse
			b.Listen("test", func(r *EventResponse) {
				resp = r
				r.Unmarshal(&received)
			})

			b.Publish(PackageInstalled, ev)
			Expect(resp.Errored()).ToNot(BeTrue())
			Expect(resp.State).Should(Equal(string(PackageInstalled)))
		})

		It("gets the json event payload", func() {
			d1 := []byte("#!/bin/bash\necho $2\n")
			err := ioutil.WriteFile(pluginFile.Name(), d1, 0550)
			Expect(err).Should(BeNil())

			m.Plugins = []Plugin{{Name: "test", Executable: pluginFile.Name()}}
			m.Events = []EventType{PackageInstalled}
			m.Subscribe(b)

			foo := map[string]string{"foo": "bar"}
			ev, err := NewEvent(PackageInstalled, foo)
			Expect(err).Should(BeNil())

			var received map[string]string
			var resp *EventResponse
			b.Listen("test", func(r *EventResponse) {
				resp = r
				r.Unmarshal(&received)
			})

			b.Publish(PackageInstalled, ev)
			Expect(resp.Errored()).ToNot(BeTrue())
			Expect(received).Should(Equal(foo))
		})
	})
})
