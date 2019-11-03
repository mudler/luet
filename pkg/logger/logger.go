package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	. "github.com/logrusorgru/aurora"

	"github.com/briandowns/spinner"
)

var s *spinner.Spinner
var m = &sync.Mutex{}

func Spinner(i int) {
	if i > 43 {
		i = 43
	}

	s = spinner.New(spinner.CharSets[i], 100*time.Millisecond) // Build our new spinner
	s.Start()                                                  // Start the spinner
}

func SpinnerText(suffix, prefix string) {
	m.Lock()
	defer m.Unlock()
	s.Suffix = Bold(Magenta(suffix)).BgBlack().String()
	s.Prefix = Bold(Cyan(prefix)).String()
}

func SpinnerStop() {
	s.Stop()
	s = nil
}

func msg(level string, msg ...interface{}) {
	var levelMsg string
	switch level {
	case "warning":
		levelMsg = Bold(Yellow("Warn")).BgBlack().String()
	case "debug":
		levelMsg = Bold(White("Debug")).BgBlack().String()
	case "info":
		levelMsg = Bold(Blue("Info")).BgBlack().String()
	case "error":
		levelMsg = Bold(Red("Error")).BgBlack().String()
	}

	if s != nil {
		SpinnerText(Sprintf(msg), levelMsg)
		return
	}

	cmd := []interface{}{levelMsg}
	for _, f := range msg {
		cmd = append(cmd, f)
	}

	fmt.Println(cmd...)
}

func Warning(mess ...interface{}) {
	msg("warning", mess)
}

func Debug(mess ...interface{}) {
	msg("debug", mess)
}

func Info(mess ...interface{}) {
	msg("info", mess)
}

func Error(mess ...interface{}) {
	msg("error", mess)
}

func Fatal(mess ...interface{}) {
	Error(mess)
	os.Exit(1)
}
