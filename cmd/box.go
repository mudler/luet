// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package cmd

import (
	. "github.com/mudler/luet/cmd/box"

	"github.com/spf13/cobra"
)

var boxGroupCmd = &cobra.Command{
	Use:   "box [command] [OPTIONS]",
	Short: "Manage luet boxes",
}

func init() {
	RootCmd.AddCommand(boxGroupCmd)

	boxGroupCmd.AddCommand(
		NewBoxExecCommand(),
	)
}
