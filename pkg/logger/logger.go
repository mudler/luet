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
var enabled = false

func Spinner(i int) {
	m.Lock()
	defer m.Unlock()
	if i > 43 {
		i = 43
	}

	if s == nil {
		s = spinner.New(spinner.CharSets[i], 100*time.Millisecond) // Build our new spinner
	}
	enabled = true
	s.Start() // Start the spinner
}

func SpinnerText(suffix, prefix string) {
	m.Lock()
	defer m.Unlock()
	if s == nil {
		s = spinner.New(spinner.CharSets[22], 100*time.Millisecond) // Build our new spinner
	}
	s.Suffix = Bold(Magenta(suffix)).BgBlack().String()
	s.Prefix = Bold(Cyan(prefix)).String()
}

func SpinnerStop() {
	m.Lock()
	defer m.Unlock()
	s.Stop()
	enabled = false
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

	if enabled {
		SpinnerText(Sprintf(msg), levelMsg)
		return
	}

	cmd := []interface{}{levelMsg}
	for _, f := range msg {
		cmd = append(cmd, f)
	}
	m.Lock()
	defer m.Unlock()
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
