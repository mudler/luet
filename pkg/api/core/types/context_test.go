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

package types_test

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gookit/color"
	types "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo"
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
	ctx := types.NewContext()

	BeforeEach(func() {
		ctx = types.NewContext()
	})

	Context("LogLevel", func() {
		It("converts it correctly to number and zaplog", func() {
			Expect(types.ErrorLevel.ToNumber()).To(Equal(0))
			Expect(types.InfoLevel.ToNumber()).To(Equal(2))
			Expect(types.WarningLevel.ToNumber()).To(Equal(1))
			Expect(types.LogLevel("foo").ToNumber()).To(Equal(3))
			Expect(types.WarningLevel.ZapLevel().String()).To(Equal("warn"))
			Expect(types.InfoLevel.ZapLevel().String()).To(Equal("info"))
			Expect(types.ErrorLevel.ZapLevel().String()).To(Equal("error"))
			Expect(types.FatalLevel.ZapLevel().String()).To(Equal("fatal"))
			Expect(types.LogLevel("foo").ZapLevel().String()).To(Equal("debug"))
		})
	})

	Context("Context", func() {
		It("detect if is a terminal", func() {
			Expect(captureStdout(func(w io.Writer) {
				_, _, err := ctx.GetTerminalSize()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("size not detectable"))
				os.Stdout.Write([]byte(err.Error()))
			})).To(ContainSubstring("size not detectable"))
		})

		It("respects loglevel", func() {
			ctx.Config.GetGeneral().Debug = false
			Expect(captureStdout(func(w io.Writer) {
				ctx.Debug("")
			})).To(Equal(""))

			ctx.Config.GetGeneral().Debug = true
			Expect(captureStdout(func(w io.Writer) {
				ctx.Debug("foo")
			})).To(ContainSubstring("foo"))
		})

		It("logs to file", func() {
			ctx.NoColor()

			t, err := ioutil.TempFile("", "tree")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(t.Name()) // clean up
			ctx.Config.GetLogging().EnableLogFile = true
			ctx.Config.GetLogging().Path = t.Name()

			ctx.Init()

			Expect(captureStdout(func(w io.Writer) {
				ctx.Info("foot")
			})).To(And(ContainSubstring("INFO"), ContainSubstring("foot")))

			Expect(captureStdout(func(w io.Writer) {
				ctx.Success("test")
			})).To(And(ContainSubstring("SUCCESS"), ContainSubstring("test")))

			Expect(captureStdout(func(w io.Writer) {
				ctx.Error("foobar")
			})).To(And(ContainSubstring("ERROR"), ContainSubstring("foobar")))

			Expect(captureStdout(func(w io.Writer) {
				ctx.Warning("foowarn")
			})).To(And(ContainSubstring("WARNING"), ContainSubstring("foowarn")))

			l, err := ioutil.ReadFile(t.Name())
			Expect(err).ToNot(HaveOccurred())
			logs := string(l)
			Expect(logs).To(ContainSubstring("foot"))
			Expect(logs).To(ContainSubstring("test"))
			Expect(logs).To(ContainSubstring("foowarn"))
			Expect(logs).To(ContainSubstring("foobar"))
		})
	})
})
