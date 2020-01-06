package logger

import (
	"fmt"
	"os"

	. "github.com/mudler/luet/pkg/config"

	"github.com/briandowns/spinner"
	"github.com/kyokomi/emoji"
	. "github.com/logrusorgru/aurora"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var s *spinner.Spinner = nil
var z *zap.Logger = nil

func NewSpinner() {
	if s == nil {
		s = spinner.New(
			spinner.CharSets[LuetCfg.GetGeneral().SpinnerCharset],
			LuetCfg.GetGeneral().GetSpinnerMs())
	}
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

	if i > 43 {
		i = 43
	}

	if !LuetCfg.GetGeneral().Debug && !s.Active() {
		//	s.UpdateCharSet(spinner.CharSets[i])
		s.Start() // Start the spinner
	}
}

func SpinnerText(suffix, prefix string) {
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

func SpinnerStop() {
	if !LuetCfg.GetGeneral().Debug {
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

func msg(level string, msg ...interface{}) {
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
	switch level {
	case "warning":
		levelMsg = Bold(Yellow(":construction: " + message)).BgBlack().String()
	case "debug":
		levelMsg = White(message).BgBlack().String()
	case "info":
		levelMsg = Bold(White(message)).BgBlack().String()
	case "error":
		levelMsg = Bold(Red(":bomb: " + message + ":fire:")).BgBlack().String()
	}

	levelMsg = emoji.Sprint(levelMsg)

	if z != nil {
		log2File(level, message)
	}

	fmt.Println(levelMsg)
}

func Warning(mess ...interface{}) {
	msg("warning", mess...)
	if LuetCfg.GetGeneral().FatalWarns {
		os.Exit(2)
	}
}

func Debug(mess ...interface{}) {
	msg("debug", mess...)
}

func Info(mess ...interface{}) {
	msg("info", mess...)
}

func Error(mess ...interface{}) {
	msg("error", mess...)
}

func Fatal(mess ...interface{}) {
	Error(mess)
	os.Exit(1)
}
