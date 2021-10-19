// +build !windows,!nacl,!plan9

package terminal

import (
	"io"
	"os"
)

func IsTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return isTerminal(int(v.Fd()))
	default:
		return false
	}
}
