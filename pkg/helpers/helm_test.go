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

package helpers_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/mudler/luet/pkg/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func writeFile(path string, content string) {
	err := ioutil.WriteFile(path, []byte(content), 0644)
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("Helm", func() {
	Context("RenderHelm", func() {
		It("Renders templates", func() {
			out, err := RenderHelm(ChartFileS("{{.Values.Test}}{{.Values.Bar}}"), map[string]interface{}{"Test": "foo"}, map[string]interface{}{"Bar": "bar"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("foobar"))
		})
		It("Renders templates with overrides", func() {
			out, err := RenderHelm(ChartFileS("{{.Values.Test}}{{.Values.Bar}}"), map[string]interface{}{"Test": "foo", "Bar": "baz"}, map[string]interface{}{"Bar": "bar"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("foobar"))
		})

		It("Renders templates", func() {
			out, err := RenderHelm(ChartFileS("{{.Values.Test}}{{.Values.Bar}}"), map[string]interface{}{"Test": "foo", "Bar": "bar"}, map[string]interface{}{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("foobar"))
		})

		It("Render files default overrides", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			toTemplate := filepath.Join(testDir, "totemplate.yaml")
			values := filepath.Join(testDir, "values.yaml")
			d := filepath.Join(testDir, "default.yaml")

			writeFile(toTemplate, `{{.Values.foo}}`)
			writeFile(values, `
foo: "bar"
`)
			writeFile(d, `
foo: "baz"
`)

			Expect(err).ToNot(HaveOccurred())

			res, err := RenderFiles(ChartFile(toTemplate), values, d)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("baz"))

		})

		It("Render files from values", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			toTemplate := filepath.Join(testDir, "totemplate.yaml")
			values := filepath.Join(testDir, "values.yaml")
			d := filepath.Join(testDir, "default.yaml")

			writeFile(toTemplate, `{{.Values.foo}}`)
			writeFile(values, `
foo: "bar"
`)
			writeFile(d, `
faa: "baz"
`)

			Expect(err).ToNot(HaveOccurred())

			res, err := RenderFiles(ChartFile(toTemplate), values, d)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("bar"))

		})

		It("Render files from values if no default", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			toTemplate := filepath.Join(testDir, "totemplate.yaml")
			values := filepath.Join(testDir, "values.yaml")

			writeFile(toTemplate, `{{.Values.foo}}`)
			writeFile(values, `
foo: "bar"
`)

			Expect(err).ToNot(HaveOccurred())

			res, err := RenderFiles(ChartFile(toTemplate), values)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("bar"))
		})

		It("Render files merging defaults", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			toTemplate := filepath.Join(testDir, "totemplate.yaml")
			values := filepath.Join(testDir, "values.yaml")
			d := filepath.Join(testDir, "default.yaml")
			d2 := filepath.Join(testDir, "default2.yaml")

			writeFile(toTemplate, `{{.Values.foo}}{{.Values.bar}}{{.Values.b}}`)
			writeFile(values, `
foo: "bar"
b: "f"
`)
			writeFile(d, `
foo: "baz"
`)

			writeFile(d2, `
foo: "do"
bar: "nei"
`)

			Expect(err).ToNot(HaveOccurred())

			res, err := RenderFiles(ChartFile(toTemplate), values, d2, d)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("bazneif"))

			res, err = RenderFiles(ChartFile(toTemplate), values, d, d2)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("doneif"))
		})

		It("doesn't interpolate if no one provides the values", func() {
			testDir, err := ioutil.TempDir(os.TempDir(), "test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(testDir)

			toTemplate := filepath.Join(testDir, "totemplate.yaml")
			values := filepath.Join(testDir, "values.yaml")
			d := filepath.Join(testDir, "default.yaml")

			writeFile(toTemplate, `{{.Values.foo}}`)
			writeFile(values, `
foao: "bar"
`)
			writeFile(d, `
faa: "baz"
`)

			Expect(err).ToNot(HaveOccurred())

			res, err := RenderFiles(ChartFile(toTemplate), values, d)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(""))

		})
	})
})
