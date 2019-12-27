package logger

import (
	"fmt"
	"os"
	"time"

	. "github.com/mudler/luet/pkg/config"

	"github.com/briandowns/spinner"
	"github.com/kyokomi/emoji"
	. "github.com/logrusorgru/aurora"
)

var s *spinner.Spinner = spinner.New(spinner.CharSets[22], 100*time.Millisecond)

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

func msg(level string, msg ...interface{}) {
	var message string
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

	// if s.Active() {
	// 	SpinnerText(levelMsg, "")
	// 	return
	// }

	cmd := []interface{}{}
	for _, f := range msg {
		cmd = append(cmd, f)
	}

	fmt.Println(levelMsg)
	//fmt.Println(cmd...)
}

func Warning(mess ...interface{}) {
	msg("warning", mess...)
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
