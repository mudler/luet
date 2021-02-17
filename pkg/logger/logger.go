package logger

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	"sync"
	. "github.com/mudler/luet/pkg/config"

	"github.com/briandowns/spinner"
	"github.com/kyokomi/emoji"
	. "github.com/logrusorgru/aurora"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var s *spinner.Spinner = nil
var z *zap.Logger = nil
var aurora Aurora = nil
var spinnerLock = sync.Mutex{}
func NewSpinner() {
	if s == nil {
		s = spinner.New(
			spinner.CharSets[LuetCfg.GetGeneral().SpinnerCharset],
			LuetCfg.GetGeneral().GetSpinnerMs())
	}
}

func InitAurora() {
	if aurora == nil {
		aurora = NewAurora(LuetCfg.GetLogging().Color)
	}
}

func GetAurora() Aurora {
	return aurora
}

func Ask() bool {
	var input string

	Info("Do you want to continue with this operation? [y/N]: ")
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

func ZapLogger() error {
	var err error
	if z == nil {
		// TODO: test permission for open logfile.
		cfg := zap.NewProductionConfig()
		cfg.OutputPaths = []string{LuetCfg.GetLogging().Path}
		cfg.Level = level2AtomicLevel(LuetCfg.GetLogging().Level)
		cfg.ErrorOutputPaths = []string{}
		if LuetCfg.GetLogging().JsonFormat {
			cfg.Encoding = "json"
		} else {
			cfg.Encoding = "console"
		}
		cfg.DisableCaller = true
		cfg.DisableStacktrace = true
		cfg.EncoderConfig.TimeKey = "time"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		z, err = cfg.Build()
		if err != nil {
			fmt.Fprint(os.Stderr, "Error on initialize file logger: "+err.Error()+"\n")
			return err
		}
	}

	return nil
}

func Spinner(i int) {
	spinnerLock.Lock()
	defer spinnerLock.Unlock()
	var confLevel int
	if LuetCfg.GetGeneral().Debug {
		confLevel = 3
	} else {
		confLevel = level2Number(LuetCfg.GetLogging().Level)
	}
	if 2 > confLevel {
		return
	}
	if i > 43 {
		i = 43
	}

	if s != nil && !s.Active() {
		//	s.UpdateCharSet(spinner.CharSets[i])
		s.Start() // Start the spinner
	}
}

func SpinnerText(suffix, prefix string) {
	if s != nil {
		s.Lock()
		defer s.Unlock()
		if LuetCfg.GetGeneral().Debug {
			fmt.Println(fmt.Sprintf("%s %s",
				Bold(Cyan(prefix)).String(),
				Bold(Magenta(suffix)).BgBlack().String(),
			))
		} else {
			s.Suffix = Bold(Magenta(suffix)).BgBlack().String()
			s.Prefix = Bold(Cyan(prefix)).String()
		}
	}
}

func SpinnerStop() {
	spinnerLock.Lock()
	defer spinnerLock.Unlock()
	var confLevel int
	if LuetCfg.GetGeneral().Debug {
		confLevel = 3
	} else {
		confLevel = level2Number(LuetCfg.GetLogging().Level)
	}
	if 2 > confLevel {
		return
	}
	if s != nil {
		s.Stop()
	}
}

func level2Number(level string) int {
	switch level {
	case "error":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

func log2File(level, msg string) {
	switch level {
	case "error":
		z.Error(msg)
	case "warning":
		z.Warn(msg)
	case "info":
		z.Info(msg)
	default:
		z.Debug(msg)
	}
}

func level2AtomicLevel(level string) zap.AtomicLevel {
	switch level {
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "warning":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "info":
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	default:
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	}
}

func Msg(level string, withoutColor, ln bool, msg ...interface{}) {
	var message string
	var confLevel, msgLevel int

	if LuetCfg.GetGeneral().Debug {
		confLevel = 3
	} else {
		confLevel = level2Number(LuetCfg.GetLogging().Level)
	}
	msgLevel = level2Number(level)
	if msgLevel > confLevel {
		return
	}

	for _, m := range msg {
		message += " " + fmt.Sprintf("%v", m)
	}

	var levelMsg string

	if withoutColor || !LuetCfg.GetLogging().Color {
		levelMsg = message
	} else {
		switch level {
		case "warning":
			levelMsg = Yellow(":construction: warning" + message).BgBlack().String()
		case "debug":
			levelMsg = White(message).BgBlack().String()
		case "info":
			levelMsg = message
		case "error":
			levelMsg = Red(message).String()
		}
	}

	if LuetCfg.GetLogging().EnableEmoji {
		levelMsg = emoji.Sprint(levelMsg)
	} else {
		re := regexp.MustCompile(`[:][\w]+[:]`)
		levelMsg = re.ReplaceAllString(levelMsg, "")
	}

	if z != nil {
		log2File(level, message)
	}

	if ln {
		fmt.Println(levelMsg)
	} else {
		fmt.Print(levelMsg)
	}

}

func Warning(mess ...interface{}) {
	Msg("warning", false, true, mess...)
	if LuetCfg.GetGeneral().FatalWarns {
		os.Exit(2)
	}
}

func Debug(mess ...interface{}) {
	pc, file, line, ok := runtime.Caller(1)
	if ok {
		mess = append([]interface{}{fmt.Sprintf("DEBUG (%s:#%d:%v)",
			path.Base(file), line, runtime.FuncForPC(pc).Name())}, mess...)
	}
	Msg("debug", false, true, mess...)
}

func DebugC(mess ...interface{}) {
	Msg("debug", true, true, mess...)
}

func Info(mess ...interface{}) {
	Msg("info", false, true, mess...)
}

func InfoC(mess ...interface{}) {
	Msg("info", true, true, mess...)
}

func Error(mess ...interface{}) {
	Msg("error", false, true, mess...)
}

func Fatal(mess ...interface{}) {
	Error(mess...)
	os.Exit(1)
}
