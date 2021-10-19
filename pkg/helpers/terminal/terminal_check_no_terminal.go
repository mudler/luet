// +build js nacl plan9

package terminal

import (
	"io"
)

func IsTerminal(w io.Writer) bool {
	return false
}
