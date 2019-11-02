package logger

import (
	"fmt"
	"time"

	. "github.com/logrusorgru/aurora"

	"github.com/briandowns/spinner"
)

var s *spinner.Spinner

func Spinner(i int) {
	if i > 43 {
		i = 43
	}

	s = spinner.New(spinner.CharSets[i], 100*time.Millisecond) // Build our new spinner
	s.Start()                                                  // Start the spinner
}

func SpinnerText(suffix, prefix string) {
	s.Suffix = Bold(Magenta(suffix)).BgBlack().String()
	s.Prefix = Bold(Cyan(prefix)).String()
}

func SpinnerStop() {
	s.Stop()
	s = nil
}

func Warning(msg ...interface{}) {
	if s != nil {
		SpinnerText(Sprintf(msg), Bold(Yellow("Warn")).BgBlack().String())
		//	return
	}
	cmd := []interface{}{Bold(Yellow("Warn")).BgBlack().String()}
	for _, f := range msg {
		cmd = append(cmd, f)
	}
	fmt.Println(cmd...)
}
func Debug(msg ...interface{}) {
	if s != nil {
		SpinnerText(Sprintf(msg), Bold(White("Debug")).BgBlack().String())
		//	return
	}
	cmd := []interface{}{Bold(White("Debug")).String()}
	for _, f := range msg {
		cmd = append(cmd, f)
	}
	fmt.Println(cmd...)
}

func Info(msg ...interface{}) {
	if s != nil {
		SpinnerText(Sprintf(msg), Bold(Blue("Info")).BgBlack().String())
		//	return
	}
	cmd := []interface{}{Bold(Green("Info")).String()}
	for _, f := range msg {
		cmd = append(cmd, f)
	}
	fmt.Println(cmd...)
}

func Error(msg ...interface{}) {
	if s != nil {
		SpinnerText(Sprintf(msg), Bold(Red("Error")).BgBlack().String())
		//	return
	}
	cmd := []interface{}{Bold(Red("Error")).String()}
	for _, f := range msg {
		cmd = append(cmd, f)
	}
	fmt.Println(cmd...)
}
