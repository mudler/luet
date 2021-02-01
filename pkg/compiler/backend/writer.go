// Copyright © 2021 Daniele Rondina <geaaru@sabayonlinux.org>
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

package backend

import (
	"bytes"

	. "github.com/mudler/luet/pkg/logger"
)

type BackendWriter struct {
	BufferedOutput bool
	Buffer         bytes.Buffer
}

func NewBackendWriter(buffered bool) *BackendWriter {
	return &BackendWriter{
		BufferedOutput: buffered,
	}
}

func (b *BackendWriter) Write(p []byte) (int, error) {
	if b.BufferedOutput {
		return b.Buffer.Write(p)
	} else {
		Msg("info", false, false, (string(p)))
	}
	return len(p), nil
}

func (b *BackendWriter) Close() error              { return nil }
func (b *BackendWriter) GetCombinedOutput() string { return b.Buffer.String() }
