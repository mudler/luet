// Copyright © 2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package logger_test

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gookit/color"
	"github.com/mudler/luet/pkg/api/core/logger"
	. "github.com/mudler/luet/pkg/api/core/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func captureStdout(f func(w io.Writer)) string {
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	color.SetOutput(w)
	f(w)

	_ = w.Close()
	out, _ := ioutil.ReadAll(r)
	os.Stdout = originalStdout
	color.SetOutput(os.Stdout)

	_ = r.Close()

	return string(out)
}

var _ = Describe("Context and logging", func() {

	Context("Context", func() {
		It("detect if is a terminal", func() {
			Expect(captureStdout(func(w io.Writer) {
				_, _, err := GetTerminalSize()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("size not detectable"))
				os.Stdout.Write([]byte(err.Error()))
			})).To(ContainSubstring("size not detectable"))
		})

		It("respects loglevel", func() {

			l, err := New(WithLevel("info"))
			Expect(err).ToNot(HaveOccurred())

			Expect(captureStdout(func(w io.Writer) {
				l.Debug("")
			})).To(Equal(""))

			l, err = New(WithLevel("debug"))
			Expect(err).ToNot(HaveOccurred())

			Expect(captureStdout(func(w io.Writer) {
				l.Debug("foo")
			})).To(ContainSubstring("foo"))
		})

		It("logs with context", func() {
			l, err := New(WithLevel("debug"), WithContext("foo"))
			Expect(err).ToNot(HaveOccurred())

			Expect(captureStdout(func(w io.Writer) {
				l.Debug("bar")
			})).To(ContainSubstring("(foo)  bar"))
		})

		It("returns copies with logged context", func() {
			l, err := New(WithLevel("debug"))
			l, _ = l.Copy(logger.WithContext("bazzz"))
			Expect(err).ToNot(HaveOccurred())

			Expect(captureStdout(func(w io.Writer) {
				l.Debug("bar")
			})).To(ContainSubstring("(bazzz)  bar"))
		})

		It("logs to file", func() {

			t, err := ioutil.TempFile("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(t.Name()) // clean up

			l, err := New(WithLevel("debug"), WithFileLogging(t.Name(), ""))
			Expect(err).ToNot(HaveOccurred())

			//	ctx.Init()

			Expect(captureStdout(func(w io.Writer) {
				l.Info("foot")
			})).To(And(ContainSubstring("INFO"), ContainSubstring("foot")))

			Expect(captureStdout(func(w io.Writer) {
				l.Success("test")
			})).To(And(ContainSubstring("SUCCESS"), ContainSubstring("test")))

			Expect(captureStdout(func(w io.Writer) {
				l.Error("foobar")
			})).To(And(ContainSubstring("ERROR"), ContainSubstring("foobar")))

			Expect(captureStdout(func(w io.Writer) {
				l.Warning("foowarn")
			})).To(And(ContainSubstring("WARNING"), ContainSubstring("foowarn")))

			ll, err := ioutil.ReadFile(t.Name())
			Expect(err).ToNot(HaveOccurred())
			logs := string(ll)
			Expect(logs).To(ContainSubstring("foot"))
			Expect(logs).To(ContainSubstring("test"))
			Expect(logs).To(ContainSubstring("foowarn"))
			Expect(logs).To(ContainSubstring("foobar"))
		})
	})
})
