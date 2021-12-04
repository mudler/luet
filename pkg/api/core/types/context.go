// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/kyokomi/emoji"
	"github.com/mudler/luet/pkg/helpers/terminal"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

const (
	ErrorLevel   LogLevel = "error"
	WarningLevel LogLevel = "warning"
	InfoLevel    LogLevel = "info"
	SuccessLevel LogLevel = "success"
	FatalLevel   LogLevel = "fatal"
)

type Context struct {
	context.Context
	Config     *LuetConfig
	IsTerminal bool
	NoSpinner  bool

	s           *pterm.SpinnerPrinter
	spinnerLock *sync.Mutex
	z           *zap.Logger
	ProgressBar *pterm.ProgressbarPrinter
}

func NewContext() *Context {
	return &Context{
		spinnerLock: &sync.Mutex{},
		IsTerminal:  terminal.IsTerminal(os.Stdout),
		Config: &LuetConfig{
			ConfigFromHost: true,
			Logging:        LuetLoggingConfig{},
			General:        LuetGeneralConfig{},
			System: LuetSystemConfig{
				DatabasePath: filepath.Join("var", "db", "packages"),
				TmpDirBase:   filepath.Join(os.TempDir(), "tmpluet")},
			Solver: LuetSolverOptions{},
		},
		s: pterm.DefaultSpinner.WithShowTimer(false).WithRemoveWhenDone(true),
	}
}

func (c *Context) Copy() *Context {

	configCopy := *c.Config
	configCopy.System = *c.Config.GetSystem()
	configCopy.General = *c.Config.GetGeneral()
	configCopy.Logging = *c.Config.GetLogging()

	ctx := *c
	ctxCopy := &ctx
	ctxCopy.Config = &configCopy

	return ctxCopy
}

// GetTerminalSize returns the width and the height of the active terminal.
func (c *Context) GetTerminalSize() (width, height int, err error) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if w <= 0 {
		w = 0
	}
	if h <= 0 {
		h = 0
	}
	if err != nil {
		err = errors.New("size not detectable")
	}
	return w, h, err
}

func (c *Context) Init() (err error) {
	if c.IsTerminal {
		if !c.Config.Logging.Color {
			c.Debug("Disabling colors")
			c.NoColor()
		}
	} else {
		c.Debug("Not a terminal, disabling colors")
		c.NoColor()
	}

	if c.Config.General.Quiet {
		c.NoColor()
		pterm.DisableStyling()
	}

	c.Debug("Colors", c.Config.GetLogging().Color)
	c.Debug("Logging level", c.Config.GetLogging().Level)
	c.Debug("Debug mode", c.Config.GetGeneral().Debug)

	if c.Config.GetLogging().EnableLogFile && c.Config.GetLogging().Path != "" {
		// Init zap logger
		err = c.InitZap()
		if err != nil {
			return
		}
	}

	// Load repositories
	err = c.Config.LoadRepositories(c)
	if err != nil {
		return
	}
	return
}

func (c *Context) NoColor() {
	pterm.DisableColor()
}

func (c *Context) Ask() bool {
	var input string

	c.Info("Do you want to continue with this operation? [y/N]: ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		return false
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true
	}
	return false
}

func (c *Context) InitZap() error {
	var err error
	if c.z == nil {
		// TODO: test permission for open logfile.
		cfg := zap.NewProductionConfig()
		cfg.OutputPaths = []string{c.Config.GetLogging().Path}
		cfg.Level = c.Config.GetLogging().Level.ZapLevel()
		cfg.ErrorOutputPaths = []string{}
		if c.Config.GetLogging().JsonFormat {
			cfg.Encoding = "json"
		} else {
			cfg.Encoding = "console"
		}
		cfg.DisableCaller = true
		cfg.DisableStacktrace = true
		cfg.EncoderConfig.TimeKey = "time"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		c.z, err = cfg.Build()
		if err != nil {
			fmt.Fprint(os.Stderr, "Error on initialize file logger: "+err.Error()+"\n")
			return err
		}
	}

	return nil
}

// Spinner starts the spinner
func (c *Context) Spinner() {
	if !c.IsTerminal || c.NoSpinner {
		return
	}

	c.spinnerLock.Lock()
	defer c.spinnerLock.Unlock()
	var confLevel int
	if c.Config.GetGeneral().Debug {
		confLevel = 3
	} else {
		confLevel = c.Config.GetLogging().Level.ToNumber()
	}
	if 2 > confLevel {
		return
	}

	if !c.s.IsActive {
		c.s, _ = c.s.Start()
	}
}

func (c *Context) Screen(text string) {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).WithMargin(2).Println(text)
	//pterm.DefaultCenter.Print(pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).WithMargin(10).Sprint(text))
}

func (c *Context) SpinnerText(suffix, prefix string) {
	if !c.IsTerminal || c.NoSpinner {
		return
	}

	c.spinnerLock.Lock()
	defer c.spinnerLock.Unlock()
	if c.Config.GetGeneral().Debug {
		fmt.Printf("%s %s\n",
			suffix, prefix,
		)
	} else {
		c.s.UpdateText(suffix + prefix)
	}
}

func (c *Context) SpinnerStop() {
	if !c.IsTerminal {
		return
	}

	c.spinnerLock.Lock()
	defer c.spinnerLock.Unlock()
	var confLevel int
	if c.Config.GetGeneral().Debug {
		confLevel = 3
	} else {
		confLevel = c.Config.GetLogging().Level.ToNumber()
	}
	if 2 > confLevel {
		return
	}
	if c.s != nil {
		c.s.Success()
	}
}

func (c *Context) log2File(level LogLevel, msg string) {
	switch level {
	case FatalLevel:
		c.z.Fatal(msg)
	case ErrorLevel:
		c.z.Error(msg)
	case WarningLevel:
		c.z.Warn(msg)
	case InfoLevel, SuccessLevel:
		c.z.Info(msg)
	default:
		c.z.Debug(msg)
	}
}

func (c *Context) Msg(level LogLevel, ln bool, msg ...interface{}) {
	var message string
	var confLevel, msgLevel int

	if c.Config.GetGeneral().Debug {
		confLevel = 3
		pterm.EnableDebugMessages()
	} else {
		confLevel = c.Config.GetLogging().Level.ToNumber()
	}
	msgLevel = level.ToNumber()

	if msgLevel > confLevel {
		return
	}

	for _, m := range msg {
		message += " " + fmt.Sprintf("%v", m)
	}

	// Color message
	levelMsg := message

	if c.Config.GetLogging().Color {
		switch level {
		case WarningLevel:
			levelMsg = pterm.LightYellow(":construction: warning" + message)
		case InfoLevel:
			levelMsg = message
		case SuccessLevel:
			levelMsg = pterm.LightGreen(message)
		case ErrorLevel:
			levelMsg = pterm.Red(message)
		default:
			levelMsg = pterm.Blue(message)
		}
	}

	// Strip emoji if needed
	if c.Config.GetLogging().EnableEmoji && c.IsTerminal {
		levelMsg = emoji.Sprint(levelMsg)
	} else {
		re := regexp.MustCompile(`[:][\w]+[:]`)
		levelMsg = re.ReplaceAllString(levelMsg, "")
	}

	if c.z != nil {
		c.log2File(level, message)
	}

	// Print the message based on the level
	switch level {
	case SuccessLevel:
		if ln {
			pterm.Success.Println(levelMsg)
		} else {
			pterm.Success.Print(levelMsg)
		}
	case InfoLevel:
		if ln {
			pterm.Info.Println(levelMsg)
		} else {
			pterm.Info.Print(levelMsg)
		}
	case WarningLevel:
		if ln {
			pterm.Warning.Println(levelMsg)
		} else {
			pterm.Warning.Print(levelMsg)
		}
	case ErrorLevel:
		if ln {
			pterm.Error.Println(levelMsg)
		} else {
			pterm.Error.Print(levelMsg)
		}
	case FatalLevel:
		if ln {
			pterm.Fatal.Println(levelMsg)
		} else {
			pterm.Fatal.Print(levelMsg)
		}
	default:
		if ln {
			pterm.Debug.Println(levelMsg)
		} else {
			pterm.Debug.Print(levelMsg)
		}
	}
}

func (c *Context) Warning(mess ...interface{}) {
	c.Msg("warning", true, mess...)
	if c.Config.GetGeneral().FatalWarns {
		os.Exit(2)
	}
}

func (c *Context) Debug(mess ...interface{}) {
	pc, file, line, ok := runtime.Caller(1)
	if ok {
		mess = append([]interface{}{fmt.Sprintf("(%s:#%d:%v)",
			path.Base(file), line, runtime.FuncForPC(pc).Name())}, mess...)
	}
	c.Msg("debug", true, mess...)
}

func (c *Context) Info(mess ...interface{}) {
	c.Msg("info", true, mess...)
}

func (c *Context) Success(mess ...interface{}) {
	c.Msg("success", true, mess...)
}

func (c *Context) Error(mess ...interface{}) {
	c.Msg("error", true, mess...)
}

func (c *Context) Fatal(mess ...interface{}) {
	c.Error(mess...)
	os.Exit(1)
}

type LogLevel string

func (level LogLevel) ToNumber() int {
	switch level {
	case ErrorLevel, FatalLevel:
		return 0
	case WarningLevel:
		return 1
	case InfoLevel, SuccessLevel:
		return 2
	default: // debug
		return 3
	}
}

func (level LogLevel) ZapLevel() zap.AtomicLevel {
	switch level {
	case FatalLevel:
		return zap.NewAtomicLevelAt(zap.FatalLevel)
	case ErrorLevel:
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case WarningLevel:
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case InfoLevel, SuccessLevel:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	default:
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	}
}
